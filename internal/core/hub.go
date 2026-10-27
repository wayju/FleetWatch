// hub.go: the Hub that holds the latest board and fans it out to /events
// subscribers.
package core

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Hub holds the latest board and wakes stream subscribers. Each subscriber
// remembers the version it last sent, so a slow client skips intermediate
// boards instead of queueing them, and never misses the newest one.
type Hub struct {
	mu      sync.RWMutex
	board   []byte
	version uint64
	at      int64
	subs    map[chan struct{}]struct{}
	maxSubs int // /events subscribers beyond this are refused with 503
}

const (
	defaultMaxSubs = 32
	writeTimeout   = 10 * time.Second // one /events write; a stalled client is dropped
)

func NewHub() *Hub { return &Hub{subs: map[chan struct{}]struct{}{}, maxSubs: defaultMaxSubs} }

// Publish stores a board; changed=false only advances the heartbeat.
func (h *Hub) Publish(board []byte, at int64, changed bool) {
	h.mu.Lock()
	if changed {
		h.board = board
		h.version++
	}
	h.at = at
	for c := range h.subs {
		select {
		case c <- struct{}{}:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) Latest() ([]byte, uint64, int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.board, h.version, h.at
}

// subscribe registers a stream, or returns nil when the cap is reached.
func (h *Hub) subscribe() chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subs) >= h.maxSubs {
		return nil
	}
	c := make(chan struct{}, 1)
	h.subs[c] = struct{}{}
	return c
}

func (h *Hub) unsubscribe(c chan struct{}) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
}

func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

func (h *Hub) serveEvents(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	c := h.subscribe()
	if c == nil {
		http.Error(w, "too many streams", http.StatusServiceUnavailable)
		return
	}
	defer h.unsubscribe(c)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	// Each write gets its own deadline, so a client that stops reading cannot
	// pin the goroutine (the server has no WriteTimeout, which would cut the
	// stream itself). Writers that cannot set one (test recorders) are skipped.
	rc := http.NewResponseController(w)
	arm := func() { _ = rc.SetWriteDeadline(time.Now().Add(writeTimeout)) }

	var sent uint64
	send := func() bool {
		board, v, at := h.Latest()
		arm()
		var err error
		if v != sent && board != nil {
			_, err = fmt.Fprintf(w, "event: board\ndata: %s\n\n", board)
			sent = v
		} else {
			_, err = fmt.Fprintf(w, "event: tick\ndata: {\"generated_at\":%d}\n\n", at)
		}
		fl.Flush()
		return err == nil
	}
	arm()
	fmt.Fprint(w, "retry: 3000\n\n")
	if !send() {
		return
	}
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-c:
			if !send() {
				return
			}
		case <-ping.C:
			arm()
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
