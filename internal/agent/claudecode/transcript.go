package claudecode

// Incremental reader for one Claude Code transcript (a session's JSONL file, or
// a subagent's). Each poll reads only the bytes appended since the last one and
// folds them into a Transcript: the latest cwd/branch/version/model, the
// context fill of the newest main-chain turn, the last few tool calls, the
// subagents it launched and the messages it sent to other sessions.
//
// Nothing here trusts the file: a line that does not parse is skipped, and a
// file that shrinks or is replaced is re-read from the start.

import (
	"bytes"
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"regexp"
	"strings"
	"time"
)

const (
	maxTools    = 12
	maxEvents   = 30
	maxMessages = 10
	maxReadPoll = 4 << 20 // bytes per poll per file; a bigger backlog finishes next tick
	maxLine     = 2 << 20 // a longer JSONL line is skipped, never buffered
	maxInitial  = 4 << 20 // a transcript first opened larger than this is read from its last 4 MiB
	textCap     = 280
)

type ToolCall struct {
	ID     string    `json:"id"`
	Name   string    `json:"name"`
	Target string    `json:"target,omitempty"`
	At     time.Time `json:"at"`
	Status string    `json:"status"` // running | ok | error
	// Description is the line Claude Code shows for the call (input.description).
	Description string `json:"description,omitempty"`
}

type AgentLaunch struct {
	ToolUseID   string `json:"tool_use_id"`
	AgentID     string `json:"agent_id,omitempty"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	// Name is the Agent call's optional name, which SendMessage may address.
	Name       string    `json:"name,omitempty"`
	Model      string    `json:"model,omitempty"`
	Background bool      `json:"background"`
	StartedAt  time.Time `json:"started_at"`
	EndedAt    time.Time `json:"ended_at,omitempty"`
	Status     string    `json:"status"` // running | parked | done | error
	prompt     string
	// HandedBackAt is when the agent last passed control back (peers.go).
	HandedBackAt time.Time `json:"-"`
	events       []agentEvent
	parked       *parkInfo // set while Status is "parked" (parked.go)
}

// Event is one entry of the activity log. Text is the label that is safe with
// FLEET_SHOW_TEXT=0 (a tool name, "prompt"); Detail is the free text that
// replaces it only when text is shown (the tool's target, the prompt itself).
type Event struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"` // tool | prompt | agent | message | turn
	Text   string    `json:"text"`
	Detail string    `json:"detail,omitempty"`
	Agent  string    `json:"agent,omitempty"`
}

type Transcript struct {
	ID            string
	Cwd           string
	Branch        string
	ClaudeVersion string
	Model         string
	Title         string
	MetaDesc      string // a subagent's description from its meta file (fleet_agents.go)
	AITitle       string // Claude Code's generated title; only a fallback for Title
	FirstPrompt   string
	LastPrompt    string
	LastReply     string
	StartedAt     time.Time
	LastActivity  time.Time
	ContextTokens int64
	// LastKind is what the newest main-chain line was, which is what the
	// transcript-only state guess reads: prompt, tool_result, tool_use, end_turn.
	LastKind  string
	Tools     []*ToolCall
	ToolCount int // every tool call seen, not just the kept tail
	Agents    []*AgentLaunch
	Messages  []board.Send
	Peers     []board.Peer // every peer line received, any sender (peers.go)
	Events    []Event

	wake         wakeState // ScheduleWakeup timer and pending background tasks (wake.go)
	pendingTools map[string]*ToolCall
	agentsByTool map[string]*AgentLaunch
	seenMsg      map[string]bool // tool_use ids already folded into Messages: a call repeats across lines
	// Older Claude Code versions write subagent turns inline as sidechain lines;
	// newer ones give each subagent its own file. Both land here, keyed by agent id.
	Sidechains map[string]*Transcript
}

func NewTranscript(id string) *Transcript {
	return &Transcript{
		ID:           id,
		pendingTools: map[string]*ToolCall{},
		agentsByTool: map[string]*AgentLaunch{},
		seenMsg:      map[string]bool{},
		Sidechains:   map[string]*Transcript{},
	}
}

type rawLine struct {
	Type             string          `json:"type"`
	SessionID        string          `json:"sessionId"`
	Cwd              string          `json:"cwd"`
	GitBranch        string          `json:"gitBranch"`
	Version          string          `json:"version"`
	Timestamp        string          `json:"timestamp"`
	IsSidechain      bool            `json:"isSidechain"`
	IsMeta           bool            `json:"isMeta"`
	Subtype          string          `json:"subtype"`
	IsCompactSummary bool            `json:"isCompactSummary"`
	AgentID          string          `json:"agentId"`
	Summary          string          `json:"summary"`
	Title            string          `json:"title"`
	CustomTitle      string          `json:"customTitle"`
	AITitle          string          `json:"aiTitle"`
	Message          *rawMessage     `json:"message"`
	ToolUseResult    json.RawMessage `json:"toolUseResult"`
	Origin           *rawOrigin      `json:"origin"`
}

type rawMessage struct {
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    json.RawMessage `json:"content"`
	Usage      *rawUsage       `json:"usage"`
	StopReason string          `json:"stop_reason"`
}

type rawUsage struct {
	InputTokens   int64 `json:"input_tokens"`
	CacheCreation int64 `json:"cache_creation_input_tokens"`
	CacheRead     int64 `json:"cache_read_input_tokens"`
	OutputTokens  int64 `json:"output_tokens"`
}

type rawBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// blocks normalises message.content, which is either a plain string or an
// array of typed blocks.
func blocks(raw json.RawMessage) []rawBlock {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return nil
		}
		return []rawBlock{{Type: "text", Text: s}}
	}
	var out []rawBlock
	if json.Unmarshal(raw, &out) != nil {
		return nil
	}
	return out
}

// contentText flattens a tool_result's content (string or blocks) to text.
func contentText(raw json.RawMessage) string {
	var b strings.Builder
	for _, blk := range blocks(raw) {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

var (
	agentIDRe  = regexp.MustCompile(`agentId:\s*([A-Za-z0-9_-]+)`)
	harnessRe  = regexp.MustCompile(`^\s*<(command-|local-command|system-reminder|task-notification|bash-|user-memory|teammate-message)`)
	agentTools = map[string]bool{"Task": true, "Agent": true}
)

// isMessageTool covers the ways one session can address another: the
// in-process agent messenger and the remote-session MCP tool.
func isMessageTool(name string) bool {
	return name == "SendMessage" || strings.HasSuffix(name, "__send_message")
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (t *Transcript) addEvent(e Event) {
	t.Events = append(t.Events, e)
	if len(t.Events) > maxEvents {
		t.Events = t.Events[len(t.Events)-maxEvents:]
	}
}

func (t *Transcript) sidechain(id string) *Transcript {
	sc := t.Sidechains[id]
	if sc == nil {
		sc = NewTranscript(id)
		t.Sidechains[id] = sc
	}
	return sc
}

// Apply folds one JSONL line. Unknown or malformed lines are ignored.
func (t *Transcript) Apply(line []byte) {
	var l rawLine
	if json.Unmarshal(line, &l) != nil {
		return
	}
	if l.IsSidechain && l.AgentID != "" && l.AgentID != t.ID {
		t.sidechain(l.AgentID).Apply(line)
		return
	}
	switch {
	case l.Summary != "" && l.Type == "summary":
		t.Title = l.Summary
		return
	case l.Type == "ai-title":
		t.AITitle = l.AITitle
		return
	case l.CustomTitle != "":
		t.Title = l.CustomTitle
		return
	case l.Title != "" && l.Message == nil:
		t.Title = l.Title
		return
	}
	// A compaction (including one done while idle) is the harness tidying the
	// context, not activity: it must not turn a finished turn into a new prompt
	// or restart the quiet clock.
	if (l.Type == "system" && (l.Subtype == "compact_boundary" || l.Subtype == "informational")) ||
		(l.Type == "user" && l.IsCompactSummary) {
		return
	}
	at := parseTime(l.Timestamp)
	if !at.IsZero() {
		if t.StartedAt.IsZero() {
			t.StartedAt = at
		}
		if at.After(t.LastActivity) {
			t.LastActivity = at
		}
	}
	if l.Cwd != "" {
		t.Cwd = l.Cwd
	}
	if l.GitBranch != "" {
		t.Branch = l.GitBranch
	}
	if l.Version != "" {
		t.ClaudeVersion = l.Version
	}
	if l.Message == nil {
		return
	}
	switch l.Type {
	case "assistant":
		t.applyAssistant(l, at)
	case "user":
		t.applyUser(l, at)
	}
}
