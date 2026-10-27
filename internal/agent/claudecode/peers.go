package claudecode

// Who is talking to whom: the lifeline of each subagent launched by a
// session, and the tool a session is running right now.
//
// A subagent's hand-back reaches the parent transcript as a meta user line with
// origin {kind: "peer", from: <agentId>, handback: bool}. A message from the
// parent to the agent is a SendMessage tool_use. Only metadata is kept here;
// the body of a peer message is never read.

import (
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"regexp"
	"time"
)

const (
	maxAgentEvents = 8
	descCap        = 140
	maxPeers       = 20
)

// agentEvent kinds: launch | to (parent to agent) | from (agent to parent, not
// a hand-back) | handback | done | error.
type agentEvent struct {
	Kind string
	At   time.Time
}

type rawOrigin struct {
	Kind     string `json:"kind"`
	From     string `json:"from"`
	Handback bool   `json:"handback"`
}

var noteStatusRe = regexp.MustCompile(`<status>\s*([A-Za-z_-]+)\s*</status>`)

// toolDescription is the line Claude Code shows for a tool call.
func toolDescription(in map[string]any) string {
	return textsafe.Clip(textsafe.Redact(str(in, "description")), descCap)
}

func (a *AgentLaunch) addEvent(kind string, at time.Time) {
	a.events = append(a.events, agentEvent{Kind: kind, At: at})
	if len(a.events) > maxAgentEvents {
		a.events = a.events[len(a.events)-maxAgentEvents:]
	}
}

func (t *Transcript) agentByID(id string) *AgentLaunch {
	if id == "" {
		return nil
	}
	for _, a := range t.Agents {
		if a.AgentID == id {
			return a
		}
	}
	return nil
}

// applyPeer folds a peer line. A hand-back also ends an agent still marked running.
func (t *Transcript) applyPeer(o *rawOrigin, at time.Time) {
	if o.From != "" {
		t.Peers = append(t.Peers, board.Peer{From: o.From, At: at, Handback: o.Handback})
		if len(t.Peers) > maxPeers {
			t.Peers = t.Peers[len(t.Peers)-maxPeers:]
		}
	}
	a := t.agentByID(o.From)
	if a == nil {
		return
	}
	if !o.Handback {
		a.addEvent("from", at)
		return
	}
	a.addEvent("handback", at)
	a.HandedBackAt = at
	if a.Status == "running" || a.Status == "parked" {
		a.Status, a.EndedAt, a.parked = "done", at, nil
	}
}

// finish ends a running agent and records why.
func (a *AgentLaunch) finish(status string, at time.Time) {
	a.Status, a.EndedAt = status, at
	a.addEvent(map[string]string{"done": "done", "error": "error"}[status], at)
}

// noteStatus maps a task-notification's status to an agent status.
func noteStatus(txt string) string {
	if m := noteStatusRe.FindStringSubmatch(txt); m != nil {
		switch m[1] {
		case "failed", "error", "killed", "stopped":
			return "error"
		}
	}
	return "done"
}

// RunningTool is the newest tool call still waiting on a result, or nil.
func (t *Transcript) RunningTool() *ToolCall {
	for i := len(t.Tools) - 1; i >= 0; i-- {
		if t.Tools[i].Status == "running" {
			return t.Tools[i]
		}
	}
	return nil
}

// eventsView is the capped lifeline for the board; never nil.
func eventsView(a *AgentLaunch) []board.AgentEventView {
	out := []board.AgentEventView{}
	for _, e := range a.events {
		out = append(out, board.AgentEventView{Kind: e.Kind, At: board.Unix(e.At)})
	}
	return out
}

// noteEnd records the end of an agent the view itself found finished (its own
// transcript reached end_turn), unless the lifeline already shows an end.
func noteEnd(av *board.AgentView, kind string) {
	if n := len(av.Events); n > 0 {
		switch av.Events[n-1].Kind {
		case "done", "error", "handback":
			return
		}
	}
	av.Events = append(av.Events, board.AgentEventView{Kind: kind, At: av.EndedAt})
	if len(av.Events) > maxAgentEvents {
		av.Events = av.Events[len(av.Events)-maxAgentEvents:]
	}
}
