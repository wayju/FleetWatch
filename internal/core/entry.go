// entry.go: Run wires the resolved configuration into a collector, a hub and
// the HTTP server. main.go is a thin shell over it.
package core

import (
	"context"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/forge"
	"log"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"time"
)

// collectorFrom builds the production collector from a resolved config: the
// runtimes and agents it names, over the shared core.
func collectorFrom(res *config.Resolved, forges *forge.Lookup, recap *Recapper) *Collector {
	rts, ags := res.Build()
	cfg := Config{Home: res.Home, Window: res.Window, Recent: res.Recent, RescanEvery: res.Rescan,
		ShowText: res.ShowText, Host: res.Host, Roots: res.WorktreeRoots, BranchPattern: res.BranchPattern, Models: res.Models}
	return newCollector(cfg, forges, recap, newRuntimeSet(rts...), ags...)
}

// startPolling begins the collection loop (local, or a hub's merge of hosts)
// publishing into hub, and returns its health record.
func startPolling(res *config.Resolved, forges *forge.Lookup, recap *Recapper, hub *Hub) *pollHealth {
	health := &pollHealth{profile: res.Profile()}
	wake := make(chan struct{}, 1)
	poke := func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
	var col *Collector
	if res.Hub == nil || res.Hub.Local {
		col = collectorFrom(res, forges, recap)
		col.rts.watch(context.Background(), poke)
	}
	if res.Hub != nil {
		h := newHubSource(res, col)
		h.start(context.Background(), poke)
		health.upstream = h.views
		go runSource(h.collect, hub, res.Poll, health, wake)
	} else {
		go run(col, hub, res.Poll, health, wake)
	}
	return health
}

// applyMemoryLimit sets the Go soft memory limit from server.memory_limit_mib.
// Zero leaves the runtime default, and an explicit GOMEMLIMIT always wins. It
// reports whether it set a limit.
func applyMemoryLimit(mib int, goMemLimit string, set func(int64) int64) bool {
	if mib <= 0 || goMemLimit != "" {
		return false
	}
	set(int64(mib) << 20)
	return true
}

// Run starts the service and returns only on a fatal error.
func Run(env config.Env) {
	res, err := config.Resolve(env)
	if err != nil {
		// A named but broken config must stop the service: falling back to a
		// default profile would serve a board that silently lacks the host's runtimes.
		log.Fatalf("fleetwatch: %v", err)
	}
	applyMemoryLimit(res.MemoryLimitMiB, os.Getenv("GOMEMLIMIT"), debug.SetMemoryLimit)
	for _, l := range res.Describe() {
		log.Print(l)
	}
	for _, w := range res.Warnings {
		log.Print(w)
	}
	every := res.Poll
	set, err := forge.NewSet(res.Forges)
	if err != nil {
		log.Fatalf("fleetwatch: %v", err)
	}
	recap := NewRecapper(res.RecapURL, os.Getenv("FLEET_RECAP_KEY"), res.RecapModel, res.RecapEvery)
	hub := NewHub()
	hub.maxSubs = res.MaxSubscribers
	token, err := resolveToken(env, log.Printf)
	if err != nil {
		log.Printf("WARNING: no token available (%v); /fleet and /events answer 503 until FLEET_TOKEN is set", err)
	}
	if w := weakTokenWarning(token); w != "" {
		log.Print(w)
	}
	health := startPolling(res, forge.NewLookup(set), recap, hub)
	mux := newMux(hub, health, every, recap)
	var sessions *sessionStore
	if res.UIEnabled {
		sessions = newSessionStore(sessionTTL, sessionCap)
	}
	registerUI(mux, uiOptions{Enabled: res.UIEnabled, ThemeFile: res.UIThemeFile}, token, sessions)

	addr := net.JoinHostPort(res.Bind, res.Port)
	log.Printf("fleetwatch on %s (home %s, poll %s, window %s, text %v, forges %d, recap %v)",
		addr, res.Home, every, res.Window, res.ShowText, len(res.Forges), recap != nil)
	srv := &http.Server{
		Addr: addr, Handler: hostGuard(res.AllowedHosts, authGuard(token, sessions, mux)),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	log.Fatal(srv.ListenAndServe())
}
