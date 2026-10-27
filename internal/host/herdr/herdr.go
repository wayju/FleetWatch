// Package herdr is the fleet's read-only adapter for herdr, a Rust agent
// multiplexer with a local socket API (see ADR-001).
//
// Protocol source: github.com/herdrdev/herdr at commit
// 2713feb56e9c9db33992db0a262c68f7426d46dd (26 October 2026, API protocol 22):
//
//	docs/next/website/src/content/docs/socket-api.mdx   transport, socket paths, methods, events
//	docs/next/website/src/content/docs/agents.mdx       how status is decided, agent explain
//	docs/next/api/herdr-api.schema.json                 request, response and event shapes
//
// What those files pin down, and what this client relies on:
//   - Newline-delimited JSON over a Unix socket, one request per line:
//     {"id","method","params"} answered by {"id","result"} or {"id","error"}.
//   - Default socket ~/.config/herdr/herdr.sock; HERDR_SOCKET_PATH overrides it.
//   - agent.list returns {"type":"agent_list","agents":[AgentInfo]}. AgentInfo
//     carries pane_id, agent, agent_status (idle|working|blocked|done|unknown),
//     state_change_seq and screen_detection_skipped. It carries no pid.
//   - pane.process_info returns the pane's foreground processes with pid, name
//     and cmdline, so a pane binds to a pid by the process the agents claim,
//     not by cwd and start time. A pane herdr cannot resolve a process for is
//     skipped rather than guessed.
//   - events.subscribe acknowledges with subscription_started, then pushes
//     {"event","data"} lines. pane.agent_status_changed needs a pane_id;
//     pane.created, pane.closed, pane.exited and pane.agent_detected do not.
//
// The agent.explain result is not typed in the schema (only documented in
// prose), so its reading in state.go is tolerant and falls back to approval.
//
// The adapter never writes to a pane: allowedMethods in client.go is the
// complete list of methods it can send. It reads no /proc file itself.
package herdr

import (
	"sync"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// RankScreen is a state herdr read off the terminal screen: a scrape, so it
// sits just above the pane rank and below the HUD projection. A state the
// agent's own hooks reported to herdr (screen_detection_skipped) is the
// agent's native word and takes host.RankNative.
const RankScreen = host.RankPane + 5

// Config is the adapter's resolved settings.
type Config struct {
	Socket  string
	Rank    int           // rank of a screen-detected state; 0 = RankScreen
	Timeout time.Duration // one request; 0 = 2s

	BackoffMin, BackoffMax time.Duration // Watch reconnect; 0 = 1s, 30s
	Logf                   func(format string, args ...any)
}

// Runtime implements host.Runtime, host.Observer and host.Watcher.
type Runtime struct {
	cfg Config

	mu      sync.Mutex
	byPID   map[int]paneState // the last Sessions pass, read by Observe
	seen    map[string]seenSeq
	explain map[explainKey]string
}

// New returns the adapter.
func New(cfg Config) *Runtime {
	if cfg.Rank == 0 {
		cfg.Rank = RankScreen
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.BackoffMin <= 0 {
		cfg.BackoffMin = time.Second
	}
	if cfg.BackoffMax <= 0 {
		cfg.BackoffMax = 30 * time.Second
	}
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	return &Runtime{cfg: cfg, byPID: map[int]paneState{}, seen: map[string]seenSeq{}, explain: map[explainKey]string{}}
}

// Name is the runtime's config type.
func (*Runtime) Name() string { return "herdr" }

type agentInfo struct {
	PaneID        string `json:"pane_id"`
	Agent         string `json:"agent"`
	Status        string `json:"agent_status"`
	Seq           uint64 `json:"state_change_seq"`
	ScreenSkipped bool   `json:"screen_detection_skipped"`
}

type process struct {
	PID     int      `json:"pid"`
	Name    string   `json:"name"`
	Cmdline string   `json:"cmdline"`
	Argv    []string `json:"argv"`
}

func (p process) command() string {
	if p.Cmdline != "" {
		return p.Cmdline
	}
	s := ""
	for i, a := range p.Argv {
		if i > 0 {
			s += " "
		}
		s += a
	}
	return s
}

// paneState is what Sessions learned about one bound pane.
type paneState struct {
	pane   string
	signal host.StateSignal
	ok     bool
}

// Sessions implements host.Runtime. A missing or unreachable socket means
// herdr is not running: no sessions, no log line.
func (r *Runtime) Sessions(now time.Time, claim host.Claimer) []host.Session {
	var list struct {
		Agents []agentInfo `json:"agents"`
	}
	if err := r.once("agent.list", struct{}{}, &list); err != nil {
		r.reset()
		return nil
	}
	var out []host.Session
	byPID := map[int]paneState{}
	live := map[string]bool{}
	for _, a := range list.Agents {
		live[a.PaneID] = true
		s, ok := r.bind(a, claim)
		if !ok {
			continue
		}
		out = append(out, s)
		sig, has := r.signal(a, now)
		byPID[s.PID] = paneState{pane: a.PaneID, signal: sig, ok: has}
	}
	r.mu.Lock()
	r.byPID = byPID
	r.prune(live)
	r.mu.Unlock()
	return out
}

// bind finds the pid in a pane's foreground whose process an agent claims.
func (r *Runtime) bind(a agentInfo, claim host.Claimer) (host.Session, bool) {
	if a.PaneID == "" {
		return host.Session{}, false
	}
	var res struct {
		Info struct {
			ShellPID int       `json:"shell_pid"`
			Procs    []process `json:"foreground_processes"`
		} `json:"process_info"`
	}
	if err := r.once("pane.process_info", map[string]string{"pane_id": a.PaneID}, &res); err != nil {
		return host.Session{}, false
	}
	for _, p := range res.Info.Procs {
		p := p
		if p.PID <= 0 {
			continue
		}
		if name, ok := claim(p.Name, p.command); ok {
			// Start is unknown to herdr: the proc runtime, listed first, owns it.
			return host.Session{Runtime: "herdr", Agent: name, PID: p.PID, PPID: res.Info.ShellPID,
				Comm: p.Name, Cmdline: p.command(), Address: a.PaneID}, true
		}
	}
	return host.Session{}, false
}

// once is one request on its own connection (see callOnce).
func (r *Runtime) once(method string, params, out any) error {
	return callOnce(r.cfg.Socket, r.cfg.Timeout, method, params, out)
}

func (r *Runtime) reset() {
	r.mu.Lock()
	r.byPID = map[int]paneState{}
	r.mu.Unlock()
}

// Observe implements host.Observer: herdr's state for the sessions it bound.
func (r *Runtime) Observe(_ time.Time, bs []host.Bound) []host.Observation {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]host.Observation, len(bs))
	for i, b := range bs {
		if ps, ok := r.byPID[b.PID]; ok && ps.ok {
			out[i].States = []host.StateSignal{ps.signal}
		}
	}
	return out
}
