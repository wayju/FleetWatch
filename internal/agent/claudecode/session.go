package claudecode

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
)

// session is one Claude Code session: its folded transcript and, on demand,
// the transcripts of its subagents.
type session struct {
	a   *Agent
	t   *Transcript
	id  string
	ref fileRef
}

var _ agent.Session = (*session)(nil)

func (s *session) ID() string        { return s.id }
func (s *session) Label() string     { return s.ref.label }
func (s *session) ConfigDir() string { return s.ref.cfgDir }

// Detail is the transcript-derived part of the session view. Tool, agent and
// message text passes through text; prompts and titles are returned raw for
// the core to gate, because it also reads them for issue references.
func (s *session) Detail(now time.Time, text func(string) string) agent.Detail {
	t := s.t
	d := agent.Detail{
		Cwd: t.Cwd, Branch: t.Branch, Version: t.ClaudeVersion, Model: t.Model, Title: t.Title,
		FirstPrompt: t.FirstPrompt, LastPrompt: t.LastPrompt, LastReply: t.LastReply,
		StartedAt: t.StartedAt, LastActivity: t.LastActivity, Context: contextOf(s.a.cfg.Models, t),
		Tools: []board.ToolView{}, Cost: board.EmptyCost(s.a.cost.AsOf()),
	}
	if d.Title == "" {
		d.Title = t.AITitle // custom title and summary win; ai-title is the generated fallback
	}
	if s.ref.path != "" {
		d.Cost = s.a.cost.Session(s.ref.path, now)
	}
	for _, tc := range t.Tools {
		d.Tools = append(d.Tools, board.ToolView{Name: tc.Name, Target: text(tc.Target), Description: text(tc.Description), At: board.Unix(tc.At), Status: tc.Status})
	}
	if rt := t.RunningTool(); rt != nil {
		d.Current = &board.ToolView{Name: rt.Name, Target: text(rt.Target), Description: text(rt.Description), At: board.Unix(rt.At), Status: rt.Status}
	}
	d.Agents = AgentsOf(now, s.a.cfg.Models, t, s.a.subagents(s.id))
	d.Messages = MainLinks(s.id, t, d.Agents, text)
	var finished map[string]bool
	d.AgentTotals, finished = s.a.agentTotals(s.ref.path, now)
	for i := range d.Agents {
		a := &d.Agents[i]
		a.Description = text(a.Description)
		if finished[a.ID] && (a.Status == "running" || a.Status == "stalled") {
			a.Status = "done"
			if a.EndedAt == 0 {
				a.EndedAt = a.LastActive
			}
		}
		if a.Current != nil {
			a.Current.Target = text(a.Current.Target)
			a.Current.Description = text(a.Current.Description)
		}
	}
	return d
}

// State is the floor signal: the transcript's own shape.
func (s *session) State(now time.Time) host.StateSignal {
	return host.StateSignal{State: TranscriptState(now, s.t), Source: "transcript", Rank: host.RankTranscript, ObservedAt: s.t.LastActivity}
}

func (s *session) NextWake(state string, async, parked int, tasks *host.TaskCap, text func(string) string) *board.NextWake {
	return NextWake(s.t, state, async, parked, tasks, text)
}

func (s *session) Digest(v board.SessionView) string { return DigestOf(v, s.t) }

// subTranscripts are the subagents' transcripts, inline sidechains overridden
// by their own files.
func (s *session) subTranscripts() map[string]*Transcript {
	subs := map[string]*Transcript{}
	for k, v := range s.t.Sidechains {
		subs[k] = v
	}
	for k, v := range s.a.subagents(s.id) {
		subs[k] = v
	}
	return subs
}

// Log is the session's own events and its subagents'.
func (s *session) Log(showText bool) []board.LogView {
	var out []board.LogView
	for _, e := range s.t.Events {
		out = append(out, board.LogView{At: board.Unix(e.At), Kind: e.Kind, Text: eventText(e, showText)})
	}
	for aid, sub := range s.subTranscripts() {
		for _, e := range sub.Events {
			out = append(out, board.LogView{At: board.Unix(e.At), Agent: board.ShortID(aid), Kind: e.Kind, Text: eventText(e, showText)})
		}
	}
	return out
}

// Touches are the places the session and its subagents were seen working.
func (s *session) Touches() []agent.Touch {
	out := TranscriptTouches(s.t)
	for aid, sub := range s.subTranscripts() {
		for _, t := range TranscriptTouches(sub) {
			t.Agent = aid
			out = append(out, t)
		}
	}
	return out
}
