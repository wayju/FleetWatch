// server.go: the polling loop, the request guards and the HTTP routes.
package core

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/wayju/fleetwatch/internal/board"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// wakeGap lets a burst of pushed events settle into one early pass.
const wakeGap = 200 * time.Millisecond

// run polls every `every`, or sooner when wake fires (a runtime's push).
// A nil wake never fires.
func run(col *Collector, hub *Hub, every time.Duration, health *pollHealth, wake <-chan struct{}) {
	runSource(col.Collect, hub, every, health, wake)
}

// runSource is run over any producer of boards: the local collector, or a hub's
// merge of several hosts (see ADR-008).
func runSource(collect func(time.Time) Snapshot, hub *Hub, every time.Duration, health *pollHealth, wake <-chan struct{}) {
	var lastBody []byte
	for {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("collect panicked: %v", r)
					health.fail(fmt.Sprint(r))
				}
			}()
			now := time.Now()
			snap := collect(now)
			snap.GeneratedAt = 0
			body, err := json.Marshal(snap)
			if err != nil {
				health.fail(err.Error())
				return
			}
			changed := !bytes.Equal(body, lastBody)
			lastBody = body
			snap.GeneratedAt = now.Unix()
			board, _ := json.Marshal(snap)
			hub.Publish(board, now.Unix(), changed)
			health.ok(now, len(snap.Sessions))
		}()
		select {
		case <-time.After(every):
		case <-wake:
			time.Sleep(wakeGap)
			select { // pokes that arrived during the gap are covered by the next pass
			case <-wake:
			default:
			}
		}
	}
}

type pollHealth struct {
	mu       sync.Mutex
	last     time.Time
	sessions int
	err      string
	profile  map[string]any              // the effective runtimes, agents and config path; nil in tests
	upstream func() []board.UpstreamView // a hub's upstream states (see ADR-008); nil on a single host
}

func (p *pollHealth) ok(t time.Time, n int) {
	p.mu.Lock()
	p.last, p.sessions, p.err = t, n, ""
	p.mu.Unlock()
}

func (p *pollHealth) fail(e string) {
	p.mu.Lock()
	p.err = e
	p.mu.Unlock()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// hostGuard refuses any request whose Host header is not on the allowlist, so
// a page on another origin cannot reach the board by rebinding its own DNS name
// to this address.
func hostGuard(allowed []string, next http.Handler) http.Handler {
	ok := map[string]bool{}
	for _, a := range allowed {
		ok[strings.ToLower(a)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ok[strings.ToLower(r.Host)] {
			http.Error(w, "host not allowed", http.StatusMisdirectedRequest)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// tokenGuard requires `Authorization: Bearer <token>` on every route except the
// probes (/health, /liveness, /version, and the same under /v1). It fails closed: with no token
// configured the data routes answer 503 rather than serving the board open.
func tokenGuard(token string, next http.Handler) http.Handler {
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/liveness", "/version", "/v1/health", "/v1/liveness", "/v1/version":
			next.ServeHTTP(w, r)
			return
		}
		if len(want) == 0 {
			http.Error(w, "FLEET_TOKEN not configured", http.StatusServiceUnavailable)
			return
		}
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newMux(hub *Hub, health *pollHealth, every time.Duration, recap *Recapper) *http.ServeMux {
	mux := http.NewServeMux()
	// Every route is also served under /v1, so a client can pin the contract
	// before the paths change; the guards treat both forms alike.
	route := func(path string, h http.HandlerFunc) {
		mux.HandleFunc("GET "+path, h)
		mux.HandleFunc("GET /v1"+path, h)
	}
	route("/fleet", func(w http.ResponseWriter, r *http.Request) {
		board, _, _ := hub.Latest()
		if board == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "first pass not finished"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(board)
	})
	route("/events", hub.serveEvents)
	route("/liveness", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "up"})
	})
	route("/health", func(w http.ResponseWriter, r *http.Request) {
		health.mu.Lock()
		last, n, e, profile := health.last, health.sessions, health.err, health.profile
		health.mu.Unlock()
		var ups []board.UpstreamView
		if health.upstream != nil {
			ups = health.upstream()
		}
		status, code := "healthy", http.StatusOK
		if last.IsZero() || time.Since(last) > 5*every+10*time.Second {
			status, code = "unhealthy", http.StatusServiceUnavailable
		}
		writeJSON(w, code, map[string]any{
			"status": status, "last_poll": board.Unix(last), "sessions": n,
			"subscribers": hub.Subscribers(), "last_error": e, "recap": recap.Health(), "profile": profile, "upstreams": ups,
		})
	})
	route("/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, buildInfo(os.Getenv))
	})
	return mux
}
