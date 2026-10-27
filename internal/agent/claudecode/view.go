package claudecode

import (
	"fmt"
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/models"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"strings"
	"time"
)

// contextOf sizes a transcript's context from the models table (see ADR-005). A
// model the table does not know has no window: Limit stays 0 and Pct 0 until
// the statusline supplies its own figures (source "unknown").
func contextOf(tbl *models.Table, t *Transcript) board.ContextView {
	if t == nil || t.ContextTokens == 0 {
		return board.ContextView{Source: "none"}
	}
	limit := tbl.Window(t.Model, t.ContextTokens)
	if limit == 0 {
		return board.ContextView{Tokens: t.ContextTokens, Source: "unknown"}
	}
	return board.ContextView{Tokens: t.ContextTokens, Limit: limit, Pct: float64(t.ContextTokens) * 100 / float64(limit), Source: "transcript"}
}

// TranscriptState guesses from the transcript alone, for sessions no hook covers.
func TranscriptState(now time.Time, t *Transcript) string {
	quiet := now.Sub(t.LastActivity)
	switch t.LastKind {
	case "end_turn":
		if quiet > 30*time.Minute {
			return "idle"
		}
		return "waiting"
	case "tool_use":
		// A tool call with no result for a while is most often a permission
		// prompt; a long Bash can look the same, hence "likely".
		if quiet > 90*time.Second {
			return "approval"
		}
		return "working"
	case "prompt", "tool_result", "assistant":
		if quiet > 10*time.Minute {
			return "idle"
		}
		return "working"
	}
	return "idle"
}

// eventText is the log line: the free-text detail, redacted, when text is
// shown, else the bare label.
func eventText(e Event, showText bool) string {
	if !showText {
		// The recipient is free text: with text off the label is bare.
		if e.Kind == "message" {
			return "→ message"
		}
		return e.Text
	}
	if e.Detail != "" {
		return textsafe.Redact(e.Detail)
	}
	return textsafe.Redact(e.Text)
}

// DigestOf is what the model sees: already-redacted prompts and replies, the
// branch and issues, recent activity and subagents. Nothing else.
func DigestOf(v board.SessionView, t *Transcript) string {
	var b strings.Builder
	fmt.Fprintf(&b, "State: %s\n", v.State)
	if v.Repo.Name != "" || v.Branch != "" {
		fmt.Fprintf(&b, "Repo: %s  Branch: %s\n", v.Repo.Name, v.Branch)
	}
	for _, i := range v.Issues {
		fmt.Fprintf(&b, "Issue #%d %s\n", i.Number, textsafe.Redact(i.Title))
	}
	if t.Title != "" {
		fmt.Fprintf(&b, "Title: %s\n", textsafe.Redact(t.Title))
	}
	fmt.Fprintf(&b, "First request: %s\n", t.FirstPrompt)
	if t.LastPrompt != t.FirstPrompt {
		fmt.Fprintf(&b, "Latest request: %s\n", t.LastPrompt)
	}
	b.WriteString("Recent activity (oldest first):\n")
	ev := t.Events
	if len(ev) > 20 {
		ev = ev[len(ev)-20:]
	}
	for _, e := range ev {
		fmt.Fprintf(&b, "- %s: %s\n", e.Kind, eventText(e, true))
	}
	for _, a := range v.Agents {
		fmt.Fprintf(&b, "Subagent %s (%s): %s\n", a.Type, a.Status, textsafe.Redact(a.Description))
	}
	fmt.Fprintf(&b, "Latest reply: %s\n", textsafe.Redact(t.LastReply))
	return b.String()
}

var fileTools = map[string]bool{"Read": true, "Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

func TranscriptTouches(t *Transcript) []agent.Touch {
	var out []agent.Touch
	if t.Cwd != "" {
		out = append(out, agent.Touch{Path: t.Cwd, At: t.LastActivity})
	}
	for _, tc := range t.Tools {
		if fileTools[tc.Name] && strings.HasPrefix(tc.Target, "/") {
			out = append(out, agent.Touch{Path: strings.TrimSuffix(tc.Target, "…"), At: tc.At})
		}
	}
	return out
}

// longShell is the age past which a live background shell is named in the wake
// reason, so a stuck loop reads as stuck rather than as a normal wait.
const longShell = 6 * time.Hour

// ageLabel renders an age as "1d 10h" or "7h"; callers pass 6 h or more.
func ageLabel(d time.Duration) string {
	h := int(d.Hours())
	if h >= 24 {
		return fmt.Sprintf("%dd %dh", h/24, h%24)
	}
	return fmt.Sprintf("%dh", h)
}

// MainLinks is a session's own sends; a recipient that is one of its agents,
// by id or launch name, resolves to the session itself with ToAgent set. Both
// lookups read every launch in the transcript, not only the agents still on
// the board.
func MainLinks(sid string, t *Transcript, agents []board.AgentView, text func(string) string) []board.LinkView {
	typ := map[string]string{}
	named := map[string]string{} // launch name -> agent id; later launches win
	for _, a := range t.Agents {
		typ[a.AgentID] = a.Type
		if a.Name != "" && a.AgentID != "" {
			named[a.Name] = a.AgentID
		}
	}
	for _, a := range agents {
		typ[a.ID] = a.Type
	}
	var out []board.LinkView
	for _, m := range t.Messages {
		l := board.LinkView{From: sid, To: textsafe.Redact(m.To), ToLabel: textsafe.Redact(m.To), At: board.Unix(m.At), Preview: text(m.Preview), Kind: "message"}
		if ty, ok := typ[m.To]; ok && m.To != "" {
			l.To, l.ToAgent, l.ToLabel = sid, m.To, board.AgentLabel(ty, m.To)
		} else if a, ok := board.AgentByName(agents, m.To); ok {
			l.To, l.ToAgent, l.ToLabel = sid, a.ID, board.AgentLabel(a.Type, a.ID)
		} else if id, ok := named[m.To]; ok {
			l.To, l.ToAgent, l.ToLabel = sid, id, board.AgentLabel(typ[id], id)
		}
		out = append(out, l)
	}
	return out
}
