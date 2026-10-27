// Package upstream is the hub side of fleet (see ADR-008): a Source follows one other
// fleet instance's /v1/events stream, and Merge folds the boards of several
// Sources (and optionally the local host) into one Snapshot.
//
// A Source holds the newest board its upstream pushed. When the stream is lost
// it keeps that board, marked stale, for the recent window and then drops it,
// so a host that restarts blinks rather than vanishes. The bearer token is only
// ever put in the Authorization header; it is not in any log line or error text.
package upstream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/httpsafe"
)

// Upstream states on the board.
const (
	StateLive  = "live"
	StateStale = "stale"
	StateDown  = "down"
)

// Source follows one upstream.
type Source struct {
	Name, URL string
	token     string

	// Tunables, set before Run; zero means the default.
	MinBackoff, MaxBackoff time.Duration
	Idle                   time.Duration // no bytes (not even a ping) for this long: reconnect
	Client                 *http.Client
	MaxEventBytes          int // largest single event accepted; zero means config.DefaultMaxEventBytes

	mu        sync.Mutex
	snap      *board.Snapshot
	connected bool
	lastSeen  time.Time
	lostAt    time.Time
	err       string
}

func NewSource(u config.Upstream) *Source {
	return &Source{Name: u.Name, URL: u.URL, token: u.Token}
}

// Part is a Source's contribution to one merge.
type Part struct {
	Name     string
	State    string
	LastSeen time.Time
	Err      string
	Snap     *board.Snapshot // nil when there is nothing to show
}

// Part reports the Source as of now. A lost stream older than recent is down,
// and its board is released.
func (s *Source) Part(now time.Time, recent time.Duration) Part {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := Part{Name: s.Name, LastSeen: s.lastSeen, Err: s.err, Snap: s.snap}
	switch {
	case s.connected:
		p.State = StateLive
	case s.snap != nil && now.Sub(s.lostAt) <= recent:
		p.State = StateStale
	default:
		p.State, p.Snap, s.snap = StateDown, nil, nil
	}
	return p
}

// Run follows the upstream until ctx ends, calling onBoard after each new
// board. Failures back off exponentially and reset once a board arrives.
func (s *Source) Run(ctx context.Context, onBoard func()) {
	minB, maxB := dflt(s.MinBackoff, time.Second), dflt(s.MaxBackoff, 30*time.Second)
	wait := minB
	if s.Client == nil { // one transport for every reconnect, so idle connections do not pile up
		s.Client = &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 15 * time.Second}}
	}
	lastLogged := ""
	for ctx.Err() == nil {
		got, err := s.stream(ctx, onBoard)
		if got {
			wait = minB
		}
		if ctx.Err() != nil {
			return
		}
		msg := s.lose(err)
		if msg != lastLogged {
			log.Printf("hub: upstream %s lost: %s", s.Name, msg)
			lastLogged = msg
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		if wait *= 2; wait > maxB {
			wait = maxB
		}
	}
}

func (s *Source) eventLimit() int {
	if s.MaxEventBytes > 0 {
		return s.MaxEventBytes
	}
	return config.DefaultMaxEventBytes
}

func dflt(v, d time.Duration) time.Duration {
	if v > 0 {
		return v
	}
	return d
}

// lose records the end of a stream and returns the scrubbed reason.
func (s *Source) lose(err error) string {
	msg := "stream ended"
	if err != nil {
		msg = strings.ReplaceAll(err.Error(), s.token, "***")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connected || s.lostAt.IsZero() {
		s.lostAt = time.Now()
	}
	s.connected, s.err = false, msg
	return msg
}

// stream makes one connection and reads it to the end. got reports whether a
// board arrived on it.
func (s *Source) stream(ctx context.Context, onBoard func()) (got bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL+"/v1/events", nil)
	if err != nil {
		return false, fmt.Errorf("bad url")
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "text/event-stream")
	client := *s.Client // a copy, so a caller-supplied client cannot re-enable redirects
	client.CheckRedirect = httpsafe.RefuseRedirect
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	idle := time.AfterFunc(dflt(s.Idle, 60*time.Second), cancel)
	defer idle.Stop()
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), s.eventLimit())
	var event, data string
	for sc.Scan() {
		idle.Reset(dflt(s.Idle, 60*time.Second))
		line := sc.Text()
		switch {
		case line == "":
			if s.dispatch(event, data) && event == "board" {
				got = true
				onBoard()
			}
			event, data = "", ""
		case strings.HasPrefix(line, "event: "):
			event = line[len("event: "):]
		case strings.HasPrefix(line, "data: "):
			data = line[len("data: "):]
		}
	}
	if ctx.Err() != nil {
		return got, fmt.Errorf("no data for %s", dflt(s.Idle, 60*time.Second))
	}
	return got, sc.Err()
}

// dispatch applies one event and reports whether it was understood. A board
// replaces the held snapshot; a tick only proves the stream is alive.
func (s *Source) dispatch(event, data string) bool {
	switch event {
	case "board":
		var snap board.Snapshot
		if json.Unmarshal([]byte(data), &snap) != nil {
			return false
		}
		s.mu.Lock()
		s.snap, s.connected, s.err, s.lastSeen = &snap, true, "", time.Now()
		s.mu.Unlock()
		return true
	case "tick":
		s.mu.Lock()
		if s.snap != nil {
			s.connected, s.err, s.lastSeen = true, "", time.Now()
		}
		s.mu.Unlock()
		return true
	}
	return false
}
