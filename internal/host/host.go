// Package host is the boundary between the fleet core and the places agent
// processes live (a pool of isolated config slots, a tmux server, a herdr instance). A
// runtime finds agent processes and reports what it knows about them; it never
// reads transcripts. Adapters implement the interfaces here and the core
// consumes only these.
package host

import (
	"context"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
)

// Session is one agent process as a runtime sees it.
type Session struct {
	Runtime   string // adapter that owns it: "proc", "tmux", "herdr"
	Agent     string // agent whose Claims accepted the process
	PID, PPID int
	Start     time.Time
	Comm      string
	Cmdline   string
	Label     string // named configuration profile the session runs under (for example a pool slot); "" when none. Becomes SessionView.Slot.
	ConfigDir string // the agent's config dir for this process; "" = the agent's default
	Address   string // locator known at discovery (a tmux pane, a herdr id); "" when unknown
	Location  string // human label of where it runs (tmux session:window.pane); "" when unknown. Becomes SessionView.Location.
}

// Claimer asks the configured agents whether a process is theirs. cmdline is
// read only if an agent asks for it, so a process whose comm no agent wants
// costs one stat read.
type Claimer func(comm string, cmdline func() string) (agent string, ok bool)

// Runtime discovers agent processes.
type Runtime interface {
	Name() string
	Sessions(now time.Time, claim Claimer) []Session
}

// Bound is a session after the agents' exact binding (session file), before
// any guess.
type Bound struct {
	Session
	SessionID string
}

// Observation is what a runtime knows about one bound session.
type Observation struct {
	SessionID string // a binding hint for an unbound session ("" = none)
	How       string // "projection"
	Address   string // where the runtime places it ("" = no opinion)
	States    []StateSignal
	Context   *ContextSignal
	Tasks     *TaskCap // live background shells; nil = no fresh scan, zero value = fresh scan, none
	Servers   []board.ServerView
}

// Observer runs once per pass, after exact binding and before transcripts are
// tailed. bs is ordered newest process first; the result has the same length
// and order, with a zero Observation for sessions the runtime does not know.
type Observer interface {
	Observe(now time.Time, bs []Bound) []Observation
}

// ConfigDir is an agent config dir a runtime knows about (a pool slot).
type ConfigDir struct{ Path, Label string }

// ConfigDirSource lets a runtime add config dirs to an agent's transcript roots.
type ConfigDirSource interface{ ConfigDirs() []ConfigDir }

// ActivitySource answers late lookups made with the final session id
// (guesses included): did a subagent hook touch within the window.
type ActivitySource interface {
	SubagentTouched(sessionID, address string) (time.Time, bool)
}

// ContextSource serves a statusline cache keyed by address.
type ContextSource interface {
	ContextCache(address string) (float64, bool)
}

// Watcher lets a push-capable runtime (herdr) wake the poll loop early. The
// core starts every Watcher at boot and runs an extra pass when one pokes; the
// periodic poll stays as the fallback, so a Watcher that never fires costs
// nothing. Watch blocks until ctx ends and must be quiet when its source is absent.
type Watcher interface {
	Watch(ctx context.Context, poke func()) error
}

// StateSignal is one claim about a session's state, ranked by how much the
// source can be trusted. The core merges them with a fixed rule.
type StateSignal struct {
	State      string // board vocabulary: working|waiting|approval|question|idle|ended|unknown|starting
	Reason     string
	Source     string // reported as state_source: hook|projection|transcript|process
	Rank       int
	ObservedAt time.Time // zero = unknown
	FreshUntil time.Time // zero = never expires
}

// Signal ranks: the highest unexpired rank wins.
const (
	RankProcess    = 0 // "starting" with no transcript
	RankTranscript = 10
	RankPane       = 20 // a scrape of the terminal
	RankProjection = 30
	RankHook       = 40
	RankNative     = 50 // a documented state file from the agent's own plugin
)

// ContextSignal is a statusline reading of context fill.
type ContextSignal struct {
	Pct  int
	Size int64
	At   time.Time
}

// TaskCap is a fresh scan of live background shells for one process.
type TaskCap struct {
	Count  int
	Oldest int64         // unix seconds the oldest live shell started; 0 when unknown
	Age    time.Duration // the oldest shell's age at the scan's pass; 0 when unknown
}
