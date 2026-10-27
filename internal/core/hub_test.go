package core

// Hub mode end to end (see ADR-008): two in-process fleet servers behind a hub.

import (
	"context"
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/config"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// fakeFleet serves one host's board the way production does, behind its token.
// seen counts requests that carried the right bearer token.
type fakeFleet struct {
	srv  *httptest.Server
	hub  *Hub
	mu   sync.Mutex
	seen int
}

func newFakeFleet(t *testing.T, host, token string, sessions ...board.SessionView) *fakeFleet {
	t.Helper()
	f := &fakeFleet{hub: NewHub()}
	snap := board.Snapshot{Host: host, Sessions: sessions, Counts: board.Counts{Live: len(sessions), Working: len(sessions)}}
	for i := range snap.Sessions {
		snap.Sessions[i].Host, snap.Sessions[i].Live = host, true
	}
	body, _ := json.Marshal(snap)
	f.hub.Publish(body, 1, true)
	health := &pollHealth{}
	health.ok(time.Now(), len(sessions))
	mux := newMux(f.hub, health, time.Second, nil)
	f.srv = httptest.NewUnstartedServer(nil)
	guarded := tokenGuard(token, mux)
	f.srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+token {
			f.mu.Lock()
			f.seen++
			f.mu.Unlock()
		}
		guarded.ServeHTTP(w, r)
	})
	f.srv.Start()
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeFleet) authed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.seen
}

func sess(id string) board.SessionView {
	return board.SessionView{ID: id, Short: board.ShortID(id), State: "working", Issues: []board.IssueRef{}, Messages: []board.LinkView{}}
}

func eventually(t *testing.T, what string, ok func() bool) {
	t.Helper()
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		if ok() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func upstreamState(s Snapshot, name string) string {
	for _, u := range s.Upstreams {
		if u.Name == name {
			return u.State
		}
	}
	return ""
}

func TestHubMergesTwoHostsMarksStaleThenDrops(t *testing.T) {
	logs := &lockedBuf{}
	log.SetOutput(logs)
	defer log.SetOutput(io.Discard)

	const tokA, tokB = "tok-alpha-SECRET-1", "tok-beta-SECRET-2"
	a := newFakeFleet(t, "alpha", tokA, sess("same-id-0001"), sess("alpha-only-01"))
	b := newFakeFleet(t, "beta", tokB, sess("same-id-0001"), sess("beta-only-001"))

	res := &config.Resolved{Host: "hub", Recent: 700 * time.Millisecond, Hub: &config.HubSpec{Upstreams: []config.Upstream{
		{Name: "alpha", URL: a.srv.URL, Token: tokA}, {Name: "beta", URL: b.srv.URL, Token: tokB}}}}
	h := newHubSource(res, nil)
	for _, s := range h.sources {
		s.MinBackoff, s.MaxBackoff = 20*time.Millisecond, 50*time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := make(chan struct{}, 1)
	h.start(ctx, func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})

	out := NewHub()
	health := &pollHealth{upstream: h.views}
	go runSource(h.collect, out, 50*time.Millisecond, health, wake)
	snap := func() Snapshot {
		var s Snapshot
		body, _, _ := out.Latest()
		_ = json.Unmarshal(body, &s)
		return s
	}

	eventually(t, "both hosts merged", func() bool { return len(snap().Sessions) == 4 })
	s := snap()
	ids := map[string]string{}
	for _, v := range s.Sessions {
		ids[v.ID] = v.Host
		if v.Stale {
			t.Errorf("%s stale while its upstream is live", v.ID)
		}
	}
	for id, host := range map[string]string{"alpha:same-id-0001": "alpha", "beta:same-id-0001": "beta", "alpha-only-01": "alpha", "beta-only-001": "beta"} {
		if ids[id] != host {
			t.Errorf("session %q host %q, want %q (ids %v)", id, ids[id], host, ids)
		}
	}
	if s.Counts.Live != 4 || upstreamState(s, "alpha") != "live" || upstreamState(s, "beta") != "live" {
		t.Errorf("counts %+v upstreams %+v", s.Counts, s.Upstreams)
	}
	if a.authed() == 0 || b.authed() == 0 {
		t.Fatalf("token not sent: alpha %d beta %d", a.authed(), b.authed())
	}

	b.srv.CloseClientConnections()
	b.srv.Close()
	eventually(t, "beta stale", func() bool { return upstreamState(snap(), "beta") == "stale" })
	s = snap()
	stale := 0
	for _, v := range s.Sessions {
		if v.Stale != (v.Host == "beta") {
			t.Errorf("%s host %s stale %v", v.ID, v.Host, v.Stale)
		}
		if v.Stale {
			stale++
		}
	}
	if stale != 2 || len(s.Sessions) != 4 || s.Counts.Live != 2 {
		t.Errorf("after loss: %d stale of %d sessions, live %d", stale, len(s.Sessions), s.Counts.Live)
	}

	eventually(t, "beta dropped after the recent window", func() bool {
		s := snap()
		return upstreamState(s, "beta") == "down" && len(s.Sessions) == 2
	})
	if s := snap(); upstreamState(s, "alpha") != "live" {
		t.Errorf("alpha %+v", s.Upstreams)
	}

	// /v1/health reports the same states.
	rec := httptest.NewRecorder()
	newMux(out, health, 50*time.Millisecond, nil).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/health", nil))
	if !strings.Contains(rec.Body.String(), `"state":"down"`) || !strings.Contains(rec.Body.String(), `"name":"alpha"`) {
		t.Errorf("health body %s", rec.Body.String())
	}

	if l := logs.String(); !strings.Contains(l, "upstream beta lost") || strings.Contains(l, "SECRET") {
		t.Errorf("log must mention the loss and never a token:\n%s", l)
	}
	if strings.Contains(string(mustJSON(snap())), "SECRET") {
		t.Error("a token reached the served board")
	}
}

func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// A wrong token leaves the upstream down with the status, not the token, as the reason.
func TestHubWrongTokenIsDownAndQuiet(t *testing.T) {
	logs := &lockedBuf{}
	log.SetOutput(logs)
	defer log.SetOutput(io.Discard)
	a := newFakeFleet(t, "alpha", "right-token", sess("a-00000001"))
	h := newHubSource(&config.Resolved{Host: "hub", Recent: time.Second, Hub: &config.HubSpec{
		Upstreams: []config.Upstream{{Name: "alpha", URL: a.srv.URL, Token: "WRONG-SECRET"}}}}, nil)
	h.sources[0].MinBackoff = 20 * time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h.start(ctx, func() {})
	eventually(t, "401 recorded", func() bool { return strings.Contains(logs.String(), "HTTP 401") })
	v := h.views()[0]
	if v.State != "down" || !strings.Contains(v.Error, "401") || strings.Contains(v.Error+logs.String(), "SECRET") {
		t.Errorf("view %+v log %s", v, logs.String())
	}
}
