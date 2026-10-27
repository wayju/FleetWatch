// Package tmux places agent processes in tmux panes (see ADR-002). It asks the tmux
// server for its panes, maps each pane's pid to the agent process below it
// using /proc stat alone (never environ, cwd, fd or exe; see ADR-007), and can read
// the visible screen against config-defined rules to say whether the agent is
// working, waiting or asking for approval. It never sends keys or text.
package tmux

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/host/proc"
)

// maxDepth bounds the walk from an agent up to a pane's shell, so a corrupt
// process table cannot loop it.
const maxDepth = 64

// Runner runs a command and returns its stdout. Tests supply recorded output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// Config is the adapter's resolved settings.
type Config struct {
	ProcRoot    string
	Binary      string        // the tmux executable; "tmux" is looked up on PATH
	Sockets     []string      // server sockets; empty means tmux's default socket
	Timeout     time.Duration // per tmux call
	Scrape      bool
	ScrapeLines int
	Rules       Rules
	Poll        time.Duration // scrape signals stay fresh for twice this
	Run         Runner        // nil runs the real binary
}

// pane is one tmux pane as list-panes reports it.
type pane struct {
	socket string
	id     string // "%N"
	pid    int
	label  string // session:window.pane
}

// Runtime implements host.Runtime and host.Observer.
type Runtime struct {
	cfg Config

	mu    sync.Mutex
	byPID map[int]pane // agent pid -> its pane, as of the last Sessions call
}

// New builds the runtime.
func New(cfg Config) *Runtime {
	if cfg.Binary == "" {
		cfg.Binary = "tmux"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if cfg.ScrapeLines <= 0 {
		cfg.ScrapeLines = 20
	}
	if cfg.Run == nil {
		cfg.Run = execRun
	}
	return &Runtime{cfg: cfg, byPID: map[int]pane{}}
}

func execRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = &out
	err := cmd.Run()
	return out.Bytes(), err
}

// Name is the runtime's config type.
func (*Runtime) Name() string { return "tmux" }

// call runs tmux against one socket ("" = the default) with the timeout.
func (r *Runtime) call(socket string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), r.cfg.Timeout)
	defer cancel()
	if socket != "" {
		args = append([]string{"-S", socket}, args...)
	}
	return r.cfg.Run(ctx, r.cfg.Binary, args...)
}

// sockets is the configured list, or the default socket alone.
func (r *Runtime) sockets() []string {
	if len(r.cfg.Sockets) == 0 {
		return []string{""}
	}
	return r.cfg.Sockets
}

// Sessions finds the agent processes that run inside a tmux pane. A socket that
// cannot be reached (no server, no permission) is skipped, so the adapter
// reports nothing rather than failing the pass.
func (r *Runtime) Sessions(_ time.Time, claim host.Claimer) []host.Session {
	byShell := map[int]pane{}
	for _, sock := range r.sockets() {
		out, err := r.call(sock, "list-panes", "-a", "-F", listFormat)
		if err != nil {
			continue
		}
		for _, p := range parsePanes(sock, string(out)) {
			if _, dup := byShell[p.pid]; !dup {
				byShell[p.pid] = p
			}
		}
	}
	found := map[int]pane{}
	var out []host.Session
	if len(byShell) > 0 {
		for _, s := range proc.Scan(r.cfg.ProcRoot, claim) {
			p, ok := r.paneAbove(s.PID, byShell)
			if !ok {
				continue
			}
			found[s.PID] = p
			s.Runtime, s.Address, s.Location = "tmux", p.id, p.label
			out = append(out, s)
		}
	}
	r.mu.Lock()
	r.byPID = found
	r.mu.Unlock()
	return out
}

// paneAbove walks up from pid until it meets a pane's shell. A pane whose
// command is the agent itself (no shell) matches at the first step.
func (r *Runtime) paneAbove(pid int, byShell map[int]pane) (pane, bool) {
	for i := 0; i < maxDepth && pid > 1; i++ {
		if p, ok := byShell[pid]; ok {
			return p, true
		}
		parent, ok := proc.ParentOf(r.cfg.ProcRoot, pid)
		if !ok {
			return pane{}, false
		}
		pid = parent
	}
	return pane{}, false
}

// listFormat is tab-separated: pane id, pane pid, then session:window.pane.
const listFormat = "#{pane_id}\t#{pane_pid}\t#{session_name}:#{window_index}.#{pane_index}"

// parsePanes reads list-panes output, skipping lines that do not parse.
func parsePanes(socket, out string) []pane {
	var ps []pane
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(f) != 3 || !strings.HasPrefix(f[0], "%") {
			continue
		}
		pid, err := strconv.Atoi(f[1])
		if err != nil || pid <= 0 {
			continue
		}
		ps = append(ps, pane{socket: socket, id: f[0], pid: pid, label: f[2]})
	}
	return ps
}
