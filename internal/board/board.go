// Exempt from the 300-line guideline: this file is declarative wire types only.
// Package board holds the wire types of the fleet API: the Snapshot the
// collector builds and everything it carries. They are the contract between
// the core, the adapters and the dashboard; field order and tags are frozen by
// the golden snapshots.
package board

import "time"

type Snapshot struct {
	GeneratedAt int64          `json:"generated_at"`
	Host        string         `json:"host"`
	Counts      Counts         `json:"counts"`
	Sessions    []SessionView  `json:"sessions"`
	Accounts    []AccountView  `json:"accounts"`
	Branches    []BranchView   `json:"branches"`
	Worktrees   []WorktreeView `json:"worktrees"`
	Links       []LinkView     `json:"links"`
	Log         []LogView      `json:"log"`
	Cost        BoardCost      `json:"cost"`
	// Upstreams is the health of each fleet a hub merges (see ADR-008); absent on a
	// single-host board.
	Upstreams []UpstreamView `json:"upstreams,omitempty"`
}

// UpstreamView is one upstream a hub subscribes to. State is live (stream
// open), stale (stream lost; its sessions are kept, marked stale) or down
// (gone for longer than the recent window, or never reached; sessions dropped).
type UpstreamView struct {
	Name     string `json:"name"`
	State    string `json:"state"`
	LastSeen int64  `json:"last_seen"`
	Error    string `json:"error,omitempty"`
}

type Counts struct {
	Live          int `json:"live"`
	Working       int `json:"working"`
	Waiting       int `json:"waiting"`
	Approval      int `json:"approval"`
	AgentsRunning int `json:"agents_running"`
	Servers       int `json:"servers"`       // agent-started listeners across live sessions
	ServersStale  int `json:"servers_stale"` // of those, the stale ones
}

type SessionView struct {
	ID           string   `json:"id"`
	Short        string   `json:"short"`
	Title        string   `json:"title,omitempty"`
	Live         bool     `json:"live"`
	Link         string   `json:"link"`
	PID          int      `json:"pid,omitempty"`
	Slot         string   `json:"slot,omitempty"` // the named configuration profile the session runs under (for example a pool slot)
	PeerName     string   `json:"-"`              // registry name SendMessage may address; resolution only, kept out of the JSON like any free text
	Pane         string   `json:"pane,omitempty"`
	Host         string   `json:"host"`               // the machine the fleet runs on
	Runtime      string   `json:"runtime,omitempty"`  // the runtime that found the process: proc, tmux, herdr
	Address      string   `json:"address,omitempty"`  // where the runtime places it (a tmux pane, same as pane)
	Location     string   `json:"location,omitempty"` // tmux session:window.pane; separate from slot
	Account      Account  `json:"account"`
	AccountKey   string   `json:"account_key"`
	State        string   `json:"state"`
	StateReason  string   `json:"state_reason,omitempty"`
	StateSource  string   `json:"state_source"`
	StateSince   int64    `json:"state_since"`
	Cwd          string   `json:"cwd,omitempty"`
	Repo         RepoInfo `json:"repo"`
	Branch       string   `json:"branch,omitempty"`
	AgentVersion string   `json:"agent_version,omitempty"` // the coding agent's own version
	// ClaudeVersion repeats AgentVersion for 0.1 only. Deprecated: removed in 0.2; use agent_version.
	ClaudeVersion string       `json:"claude_version,omitempty"`
	Model         string       `json:"model,omitempty"`
	ModelClass    string       `json:"model_class,omitempty"` // display class from the models config
	Context       ContextView  `json:"context"`
	Issues        []IssueRef   `json:"issues"`
	Recap         *Recap       `json:"recap,omitempty"`
	Tools         []ToolView   `json:"tools"`
	Current       *ToolView    `json:"current"` // the tool running now, null when none
	Agents        []AgentView  `json:"agents"`
	AgentTotals   AgentTotals  `json:"agent_totals"`
	Messages      []LinkView   `json:"messages"`
	StartedAt     int64        `json:"started_at"`
	LastActivity  int64        `json:"last_activity"`
	Cost          SessionCost  `json:"cost"`
	NextWake      *NextWake    `json:"next_wake"`       // null unless the session will act again on its own; such a session reports state working, not waiting
	Servers       []ServerView `json:"servers"`         // listeners under the session's shells
	Stale         bool         `json:"stale,omitempty"` // hub only: the upstream carrying it is unreachable (see ADR-008)
}

type ContextView struct {
	Tokens int64   `json:"tokens"`
	Limit  int64   `json:"limit"`
	Pct    float64 `json:"pct"`
	Source string  `json:"source"` // statusline | transcript | unknown | none
	// Limit 0 means the window is unknown (no models entry, no statusline
	// size) and Pct is then only the statusline's, if any.
}

type Recap struct {
	Summary     string `json:"summary,omitempty"`    // written by the recapper, outside the session
	SummaryAt   int64  `json:"summary_at,omitempty"` // when it was written
	FirstPrompt string `json:"first_prompt,omitempty"`
	LastPrompt  string `json:"last_prompt,omitempty"`
	LastReply   string `json:"last_reply,omitempty"`
}

type ToolView struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	At     int64  `json:"at"`
	Status string `json:"status"`
	// Description is the call's own one-line summary; dropped with FLEET_SHOW_TEXT=0.
	Description string `json:"description,omitempty"`
}

type AgentView struct {
	ID          string      `json:"id,omitempty"`
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Name        string      `json:"-"` // launch name SendMessage may address; resolution only
	Model       string      `json:"model,omitempty"`
	Background  bool        `json:"background"`
	Status      string      `json:"status"` // running | done | error | stalled
	StartedAt   int64       `json:"started_at"`
	EndedAt     int64       `json:"ended_at"`
	LastActive  int64       `json:"last_active"`
	ToolCount   int         `json:"tool_count"`
	Current     *ToolView   `json:"current,omitempty"`
	Context     ContextView `json:"context"`
	// Events is the last 8 of launch|to|from|handback|done|error; HandedBackAt
	// is unix seconds, 0 if the agent never handed back.
	Events       []AgentEventView `json:"events"`
	HandedBackAt int64            `json:"handed_back_at"`

	Sends    []Send `json:"-"` // the agent's own sends (resolved into links by the core)
	Received []Peer `json:"-"` // incoming peer lines
}

type AccountView struct {
	Key      string   `json:"key"`
	Email    string   `json:"email,omitempty"`
	Name     string   `json:"name,omitempty"`
	Org      string   `json:"org,omitempty"`
	Slots    []string `json:"slots"` // slot names: the configuration profiles the account's sessions ran under; "default" when none
	Sessions int      `json:"sessions"`
	Live     int      `json:"live"`
}

type BranchView struct {
	Repo     string   `json:"repo"`
	Branch   string   `json:"branch"`
	Version  string   `json:"version,omitempty"`
	Commit   string   `json:"commit,omitempty"`
	Sessions []string `json:"sessions"`
	Shared   bool     `json:"shared"`         // more than one live session on this checkout
	Host     string   `json:"host,omitempty"` // hub only: the machine holding the checkout (see ADR-008)
}

type LinkView struct {
	From     string `json:"from"`
	To       string `json:"to"`
	ToLabel  string `json:"to_label"`
	At       int64  `json:"at"`
	Preview  string `json:"preview,omitempty"`
	External bool   `json:"external"`
	// Agent endpoints: From/To stay the session card holding the
	// endpoint; *Agent names the subagent inside it, "" when none.
	FromAgent string `json:"from_agent"`
	ToAgent   string `json:"to_agent"`
	Kind      string `json:"kind"` // message | handback
}

type LogView struct {
	At      int64  `json:"at"`
	Session string `json:"session"`
	Short   string `json:"short"`
	Agent   string `json:"agent,omitempty"`
	Kind    string `json:"kind"`
	Text    string `json:"text"`
}

// ServerView is one listening process under a session's shell. Cwd and Dir are
// host paths and Cmd is free text, so all three are dropped with FLEET_SHOW_TEXT=0.
type ServerView struct {
	PID         int    `json:"pid"`
	Port        int    `json:"port"`
	Cmd         string `json:"cmd,omitempty"`
	Cwd         string `json:"cwd,omitempty"`
	Dir         string `json:"dir,omitempty"`
	StartedAt   int64  `json:"started_at"`
	Stale       bool   `json:"stale"`
	StaleReason string `json:"stale_reason,omitempty"` // age | merged | dir_gone
}

type NextWake struct {
	Kind   string `json:"kind"` // timer | condition
	At     int64  `json:"at,omitempty"`
	Reason string `json:"reason"`
}

type AgentTotals struct {
	Total          int   `json:"total"`
	Running        int   `json:"running"`
	LastFinishedAt int64 `json:"last_finished_at"`
}

type AgentEventView struct {
	Kind string `json:"kind"`
	At   int64  `json:"at"`
}

// peerRec is one peer line received, whoever sent it; the body is not kept.
// Peer is one peer line received.
type Peer struct {
	From     string
	At       time.Time
	Handback bool
}

type WorktreeWriter struct {
	Session   string `json:"session"`
	Slot      string `json:"slot"`
	AgentID   string `json:"agent_id"`
	AgentDesc string `json:"agent_desc"`
	LastAt    int64  `json:"last_at"`
}

// WorktreeView is one entry of Snapshot.Worktrees. Name and RepoPath are
// additions to the contract: the dashboard's worktree-rm hint needs them.
type WorktreeView struct {
	Repo       string           `json:"repo"`
	RepoPath   string           `json:"repo_path"`
	Name       string           `json:"name"`
	PathRel    string           `json:"path_rel"`
	Branch     string           `json:"branch"`
	Issue      *int             `json:"issue"`
	Writers    []WorktreeWriter `json:"writers"`
	Active     bool             `json:"active"`
	LastGitAt  int64            `json:"last_git_at"`
	Stale      bool             `json:"stale"`
	Merged     *bool            `json:"merged"` // null: unknown, never guessed
	TwoWriters bool             `json:"two_writers"`
	Host       string           `json:"host,omitempty"` // hub only (see ADR-008)
}

type IssueRef struct {
	Repo   string `json:"repo,omitempty"` // owner/name
	Number int    `json:"number"`
	Source string `json:"source"` // branch | url | mention
	Title  string `json:"title,omitempty"`
	State  string `json:"state,omitempty"`
	IsPR   bool   `json:"is_pr,omitempty"`
	URL    string `json:"url,omitempty"`
}

type RepoInfo struct {
	Root       string `json:"root,omitempty"`
	Remote     string `json:"-"`
	Host       string `json:"host,omitempty"`
	Owner      string `json:"owner,omitempty"`
	Name       string `json:"name,omitempty"`
	Branch     string `json:"branch,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Version    string `json:"version,omitempty"`
	VersionSrc string `json:"version_source,omitempty"`
	Worktree   bool   `json:"worktree,omitempty"`
}

// Slug is owner/name, or "" when the remote could not be parsed.
func (r RepoInfo) Slug() string {
	if r.Owner == "" || r.Name == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

type Account struct {
	Email string `json:"email,omitempty"`
	Name  string `json:"name,omitempty"`
	Org   string `json:"org,omitempty"`
}

// Send is one SendMessage a session or agent made.
type Send struct {
	To      string    `json:"to"`
	At      time.Time `json:"at"`
	Preview string    `json:"preview,omitempty"`
}

// --- view types (unix-second timestamps; 0 means unknown) ---
// Unix is unix seconds; 0 means unknown.
func Unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// ShortID is the first 8 characters of a session id.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// AgentLabel names a subagent by type and the first 7 characters of its id.
func AgentLabel(typ, id string) string {
	if typ == "" {
		typ = "agent"
	}
	if len(id) > 7 {
		id = id[:7]
	}
	return typ + " " + id
}

// AgentByName finds a session's agent by the name it was launched with. A
// name reused across launches means the newest launch overall, as SendMessage
// resolves it now, so an older send to that name is drawn to the newer agent.
// Equal start times keep the first in slice order.
func AgentByName(agents []AgentView, name string) (AgentView, bool) {
	var best AgentView
	found := false
	for _, a := range agents {
		if name != "" && a.Name == name && a.ID != "" && (!found || a.StartedAt > best.StartedAt) {
			best, found = a, true
		}
	}
	return best, found
}
