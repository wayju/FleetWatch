package core

// Agent links: who messages whom, down to the subagent. From/To of a
// LinkView are always the session card holding the endpoint; FromAgent/ToAgent
// name the subagent inside it.

import (
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"regexp"
	"sort"
)

var agentIDLike = regexp.MustCompile(`^a[0-9a-f]{15,}$`)

type agentRef struct{ sid, typ string }

func markExternal(l *LinkView, showText bool) {
	l.External = true
	if !showText {
		l.To, l.ToLabel = "", "external"
	}
}

// agentLinks derives links from the subagents: their sends, their lifeline
// events toward the parent, and the peer lines they received.
func agentLinks(sessions []SessionView, ids map[string]string, showText bool) []LinkView {
	idx := map[string]agentRef{}
	for _, v := range sessions {
		for _, a := range v.Agents {
			if a.ID != "" {
				idx[a.ID] = agentRef{v.ID, a.Type}
			}
		}
	}
	var out []LinkView
	for _, v := range sessions {
		for _, a := range v.Agents {
			if a.ID == "" {
				continue
			}
			for _, m := range a.Sends {
				l := LinkView{From: v.ID, FromAgent: a.ID, At: board.Unix(m.At), Kind: "message"}
				if showText {
					l.Preview = m.Preview
				}
				sendTarget(&l, v, m.To, idx, ids, showText)
				out = append(out, l)
			}
			for _, e := range a.Events {
				if e.Kind == "from" || e.Kind == "handback" {
					k := "message"
					if e.Kind == "handback" {
						k = "handback"
					}
					out = append(out, LinkView{From: v.ID, FromAgent: a.ID, To: v.ID, ToLabel: board.ShortID(v.ID), At: e.At, Kind: k})
				}
			}
			for _, p := range a.Received {
				l := LinkView{To: v.ID, ToAgent: a.ID, ToLabel: board.AgentLabel(a.Type, a.ID), At: board.Unix(p.At), Kind: "message"}
				if p.Handback {
					l.Kind = "handback"
				}
				if peerSource(&l, v, a.ID, p.From, idx) {
					out = append(out, l)
				}
			}
		}
	}
	return out
}

// sendTarget sets the recipient of an agent's send: its parent ("main"), any
// agent on the board by id, a sibling by launch name, a bare agent id, or
// another session; anything else is external.
func sendTarget(l *LinkView, v SessionView, to string, idx map[string]agentRef, ids map[string]string, showText bool) {
	named, byName := board.AgentByName(v.Agents, to)
	switch r, ok := idx[to]; {
	case to == "main":
		l.To, l.ToLabel = v.ID, board.ShortID(v.ID)
	case ok:
		l.To, l.ToAgent, l.ToLabel = r.sid, to, board.AgentLabel(r.typ, to)
	case byName:
		l.To, l.ToAgent, l.ToLabel = v.ID, named.ID, board.AgentLabel(named.Type, named.ID)
	case agentIDLike.MatchString(to):
		l.To, l.ToAgent, l.ToLabel = v.ID, to, board.AgentLabel("", to)
	default:
		if id, ok := resolveSession(ids, to); ok {
			l.To, l.ToLabel = id, board.ShortID(id)
		} else {
			l.To, l.ToLabel = textsafe.Redact(to), textsafe.Redact(to)
			markExternal(l, showText)
		}
	}
}

// peerSource sets the sender of a peer line an agent received, and reports
// false when it cannot be placed or is the agent itself.
func peerSource(l *LinkView, v SessionView, self, from string, idx map[string]agentRef) bool {
	named, byName := board.AgentByName(v.Agents, from)
	switch r, ok := idx[from]; {
	case from == self:
		return false
	case from == "main":
		l.From = v.ID
	case ok:
		l.From, l.FromAgent = r.sid, from
	case byName:
		if named.ID == self {
			return false
		}
		l.From, l.FromAgent = v.ID, named.ID
	case agentIDLike.MatchString(from):
		l.From, l.FromAgent = v.ID, from
	default:
		return false
	}
	return true
}

// dedupeLinks keeps one link per (from, from_agent, to, to_agent, kind), the
// newest, in order of first appearance.
func dedupeLinks(in []LinkView) []LinkView {
	pos := map[[5]string]int{}
	var out []LinkView
	for _, l := range in {
		k := [5]string{l.From, l.FromAgent, l.To, l.ToAgent, l.Kind}
		if i, ok := pos[k]; ok {
			if l.At >= out[i].At {
				out[i] = l
			}
			continue
		}
		pos[k] = len(out)
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At < out[j].At })
	return out
}
