package claudecode

// The assistant and user message handlers of the transcript reader: tool_use
// blocks, subagent launches, tool results and the messages sent to peers.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"strings"
	"time"
)

func (t *Transcript) applyAssistant(l rawLine, at time.Time) {
	m := l.Message
	if m.Model != "" && m.Model != "<synthetic>" {
		t.Model = m.Model
	}
	if u := m.Usage; u != nil {
		if n := u.InputTokens + u.CacheCreation + u.CacheRead + u.OutputTokens; n > 0 {
			t.ContextTokens = n
		}
	}
	sawTool := false
	for _, b := range blocks(m.Content) {
		switch b.Type {
		case "text":
			if s := strings.TrimSpace(b.Text); s != "" {
				t.LastReply = textsafe.Clip(textsafe.Redact(s), textCap)
			}
		case "tool_use":
			sawTool = true
			t.applyToolUse(b, at)
		}
	}
	switch {
	case sawTool:
		t.LastKind = "tool_use"
	case m.StopReason == "end_turn" || m.StopReason == "stop_sequence":
		t.LastKind = "end_turn"
		t.addEvent(Event{At: at, Kind: "turn", Text: "turn done"})
	default:
		t.LastKind = "assistant"
	}
}

func (t *Transcript) applyToolUse(b rawBlock, at time.Time) {
	var in map[string]any
	_ = json.Unmarshal(b.Input, &in)
	tc := &ToolCall{ID: b.ID, Name: b.Name, Target: toolTarget(b.Name, in), Description: toolDescription(in), At: at, Status: "running"}
	t.ToolCount++
	t.wakeToolUse(b, in, at)
	t.Tools = append(t.Tools, tc)
	if len(t.Tools) > maxTools {
		t.Tools = t.Tools[len(t.Tools)-maxTools:]
	}
	if b.ID != "" {
		t.pendingTools[b.ID] = tc
	}
	ev := Event{At: at, Kind: "tool", Text: b.Name}
	if tc.Target != "" {
		ev.Detail = b.Name + " · " + tc.Target
	}
	switch {
	case agentTools[b.Name]:
		a := &AgentLaunch{
			ToolUseID:   b.ID,
			Type:        str(in, "subagent_type"),
			Description: str(in, "description"),
			Name:        str(in, "name"),
			Model:       str(in, "model"),
			Background:  in["run_in_background"] == true,
			StartedAt:   at,
			Status:      "running",
			prompt:      textsafe.Clip(textsafe.Redact(str(in, "prompt")), 400),
		}
		if a.Type == "" {
			a.Type = "general-purpose"
		}
		t.Agents = append(t.Agents, a)
		t.agentsByTool[b.ID] = a
		a.addEvent("launch", at)
		ev = Event{At: at, Kind: "agent", Text: "launch " + a.Type, Detail: "launch " + a.Type + ": " + a.Description}
	case isMessageTool(b.Name):
		if b.ID != "" {
			if t.seenMsg[b.ID] {
				return
			}
			if len(t.seenMsg) >= 512 {
				t.seenMsg = map[string]bool{}
			}
			t.seenMsg[b.ID] = true
		}
		to := firstStr(in, "to", "recipient", "session_id", "name")
		msg := board.Send{To: to, At: at, Preview: textsafe.Clip(textsafe.Redact(firstStr(in, "message", "content", "text")), 120)}
		t.Messages = append(t.Messages, msg)
		if a := t.agentByID(to); a != nil {
			a.addEvent("to", at)
		}
		if len(t.Messages) > maxMessages {
			t.Messages = t.Messages[len(t.Messages)-maxMessages:]
		}
		ev = Event{At: at, Kind: "message", Text: "→ " + to, Detail: "→ " + to + ": " + msg.Preview}
	}
	t.addEvent(ev)
}

func (t *Transcript) applyUser(l rawLine, at time.Time) {
	if l.Origin != nil && l.Origin.Kind == "peer" {
		t.applyPeer(l.Origin, at)
		return
	}
	kind := ""
	for _, b := range blocks(l.Message.Content) {
		switch b.Type {
		case "tool_result":
			kind = "tool_result"
			t.applyToolResult(b, l.ToolUseResult, at)
		case "text":
			txt := strings.TrimSpace(b.Text)
			if txt == "" {
				continue
			}
			// Harness-injected text (reminders, slash-command echoes, background
			// task notifications) is not the human talking; a notification can
			// still finish a background agent it names.
			if l.IsMeta || harnessRe.MatchString(txt) {
				t.noteAgents(txt, at)
				t.wakeNotification(txt)
				t.wakeUser(txt, true, at)
				continue
			}
			kind = "prompt"
			t.wakeUser(txt, false, at)
			p := textsafe.Clip(textsafe.Redact(txt), textCap)
			if t.FirstPrompt == "" {
				t.FirstPrompt = p
			}
			t.LastPrompt = p
			t.addEvent(Event{At: at, Kind: "prompt", Text: "prompt", Detail: textsafe.Clip(p, 120)})
		}
	}
	if kind != "" {
		t.LastKind = kind
	}
}

func (t *Transcript) applyToolResult(b rawBlock, extra json.RawMessage, at time.Time) {
	t.wakeToolResult(b, extra)
	if tc := t.pendingTools[b.ToolUseID]; tc != nil {
		tc.Status = "ok"
		if b.IsError {
			tc.Status = "error"
		}
		delete(t.pendingTools, b.ToolUseID)
	}
	a := t.agentsByTool[b.ToolUseID]
	if a == nil {
		return
	}
	var meta struct {
		AgentID string `json:"agentId"`
		IsAsync bool   `json:"isAsync"`
		Status  string `json:"status"`
	}
	_ = json.Unmarshal(extra, &meta)
	if a.AgentID == "" {
		a.AgentID = meta.AgentID
		if a.AgentID == "" {
			if m := agentIDRe.FindStringSubmatch(contentText(b.Content)); m != nil {
				a.AgentID = m[1]
			}
		}
	}
	// Claude Code launches agents asynchronously by default, without
	// run_in_background in the input; the receipt says so.
	if meta.IsAsync || meta.Status == "async_launched" {
		a.Background = true
		if len(a.events) == 1 && a.events[0].Kind == "launch" {
			a.events[0].At = at // the receipt is when the agent really started
		}
	}
	// A background launch returns at once; its result is the receipt, not
	// the end. It finishes when a notification names it or its own
	// transcript reaches end_turn.
	if a.Background && !b.IsError {
		return
	}
	status := "done"
	if b.IsError {
		status = "error"
	}
	a.finish(status, at)
}

// PendingTools is how many tool calls are waiting on a result.
func (t *Transcript) PendingTools() int { return len(t.pendingTools) }
