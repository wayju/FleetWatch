// Package agent is the boundary between the fleet core and the kind of agent
// running in a process (Claude Code today). An agent claims processes, binds
// them to sessions, and folds its own transcripts into the views the core
// assembles.
package agent

import (
	"time"

	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
)

// BindMode says how much evidence a binding needs.
type BindMode int

const (
	// BindExact is proof only: claudecode reads <ConfigDir>/sessions/<pid>.json.
	BindExact BindMode = iota
	// BindGuess takes the newest unclaimed transcript active since the
	// process started (less a minute), in the same label.
	BindGuess
)

// Binding is the session an agent found for a process.
type Binding struct{ SessionID, Peer, How string } // How: session-file | guess

// Agent owns transcripts and everything derived from them.
type Agent interface {
	Name() string
	Claims(comm string, cmdline func() string) bool
	Bind(s host.Session, mode BindMode, claimed map[string]bool) (Binding, bool)
	// Tail rescans the transcript index on its cadence and polls every
	// transcript touched within the window plus every claimed id, and their
	// subagent files.
	Tail(now time.Time, claimed map[string]bool)
	Session(id string) Session                                                   // nil when no transcript is held
	Recent(now time.Time, recent time.Duration, linked map[string]bool) []string // ended, ordered by id
	Account(configDir string) board.Account                                      // "" = default config dir
	EndPass(now time.Time)                                                       // agent totals prune, cost prune
}

// Session is one agent session's transcript, folded.
type Session interface {
	ID() string
	Label() string // config-dir label holding the transcript (an ended session's slot)
	ConfigDir() string
	Detail(now time.Time, text func(string) string) Detail // every free-text field passes through text
	State(now time.Time) host.StateSignal                  // the transcript floor, RankTranscript
	NextWake(state string, async, parked int, tasks *host.TaskCap, text func(string) string) *board.NextWake
	Digest(v board.SessionView) string // recap input
	Log(showText bool) []board.LogView // own and subagent events; core fills Session/Short
	Touches() []Touch                  // worktree writers
}

// Detail is the transcript-derived part of a session view.
type Detail struct {
	Cwd, Branch, Version, Model, Title string
	FirstPrompt, LastPrompt, LastReply string
	StartedAt, LastActivity            time.Time
	Context                            board.ContextView // transcript-derived; core applies statusline over it
	Tools                              []board.ToolView
	Current                            *board.ToolView
	Agents                             []board.AgentView // finished-by-meta already applied
	Messages                           []board.LinkView  // recipients unresolved; finish resolves them
	AgentTotals                        board.AgentTotals
	Cost                               board.SessionCost
}

// Touch is a place a session or agent was seen working.
type Touch struct {
	Agent, Path string // Agent "" = main thread
	At          time.Time
}
