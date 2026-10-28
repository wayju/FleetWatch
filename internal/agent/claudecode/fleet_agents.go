package claudecode

// Subagent views joined from the parent transcript and sidechain files.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/models"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	AgentStall    = 10 * time.Minute
	agentKeepDone = 15 * time.Minute
	MaxAgents     = 8
)

// AgentsOf joins the launches recorded in the parent transcript with the
// subagents' own transcripts (files or inline sidechains). A subagent whose
// launch fell outside the window still shows, as type "agent".
func AgentsOf(now time.Time, tbl *models.Table, t *Transcript, files map[string]*Transcript) []board.AgentView {
	subs := map[string]*Transcript{}
	for id, s := range t.Sidechains {
		subs[id] = s
	}
	for id, s := range files {
		subs[id] = s
	}
	used := map[string]bool{}
	var out []board.AgentView
	for _, a := range t.Agents {
		sub := matchSub(a, subs, used)
		av := board.AgentView{ID: a.AgentID, Type: a.Type, Description: a.Description, Name: a.Name, Model: a.Model,
			Background: a.Background, Status: a.Status, StartedAt: board.Unix(a.StartedAt), EndedAt: board.Unix(a.EndedAt),
			Events: eventsView(a), HandedBackAt: board.Unix(a.HandedBackAt)}
		if sub != nil {
			used[sub.ID] = true
			av.ID = sub.ID
		}
		a.parkView(now, sub, &av)
		if a.wokenBy(sub) {
			av.Status, av.EndedAt = "running", 0
		}
		if sub != nil {
			fillFromSub(now, tbl, &av, sub)
		}
		out = append(out, av)
	}
	for id, s := range subs {
		if used[id] {
			continue
		}
		av := board.AgentView{ID: id, Type: "agent", Status: "running", StartedAt: board.Unix(s.StartedAt), Events: []board.AgentEventView{}}
		av.Description = textsafe.Clip(s.FirstPrompt, 80)
		if s.MetaDesc != "" {
			av.Description = s.MetaDesc
		}
		fillFromSub(now, tbl, &av, s)
		out = append(out, av)
	}
	return keepRecent(now, out)
}

// keepRecent keeps running agents and the recently finished, newest first,
// capped at MaxAgents; never nil.
func keepRecent(now time.Time, out []board.AgentView) []board.AgentView {
	var keep []board.AgentView
	for _, a := range out {
		end := a.EndedAt
		if end == 0 {
			end = a.LastActive
		}
		if a.Status == "running" || a.Status == "stalled" || a.Status == "parked" || now.Unix()-end <= int64(agentKeepDone/time.Second) {
			keep = append(keep, a)
		}
	}
	// Working agents first: a re-woken old agent has the oldest start time and
	// must not be the one the cap drops.
	sort.SliceStable(keep, func(i, j int) bool {
		ri, rj := keep[i].Status != "done" && keep[i].Status != "error", keep[j].Status != "done" && keep[j].Status != "error"
		if ri != rj {
			return ri
		}
		if keep[i].StartedAt != keep[j].StartedAt {
			return keep[i].StartedAt > keep[j].StartedAt
		}
		return keep[i].ID < keep[j].ID
	})
	if len(keep) > MaxAgents {
		keep = keep[:MaxAgents]
	}
	if keep == nil {
		keep = []board.AgentView{}
	}
	return keep
}

func fillFromSub(now time.Time, tbl *models.Table, av *board.AgentView, s *Transcript) {
	av.Sends, av.Received = s.Messages, s.Peers
	av.LastActive = board.Unix(s.LastActivity)
	av.ToolCount = s.ToolCount
	if n := len(s.Tools); n > 0 {
		tc := s.Tools[n-1]
		av.Current = &board.ToolView{Name: tc.Name, Target: tc.Target, Description: tc.Description, At: board.Unix(tc.At), Status: tc.Status}
	}
	if av.Model == "" {
		av.Model = s.Model
	}
	av.Context = contextOf(tbl, s)
	if av.Status == "running" {
		switch {
		case s.LastKind == "end_turn" && s.PendingTools() == 0:
			av.Status, av.EndedAt = "done", board.Unix(s.LastActivity)
			noteEnd(av, "done")
		case !s.LastActivity.IsZero() && now.Sub(s.LastActivity) > AgentStall:
			av.Status = "stalled"
		}
	}
}

// wokenSlack absorbs the gap between an agent's last transcript line and the
// hand-back or notification the parent records for it a moment later.
const wokenSlack = 2 * time.Second

// wokenBy reports whether a finished agent is working again: SendMessage
// resumes an agent from its transcript after its hand-back or notification,
// so its own transcript moving past the end means it is running. fillFromSub
// ends it again at its next end_turn.
func (a *AgentLaunch) wokenBy(sub *Transcript) bool {
	if sub == nil || a.parked != nil || a.EndedAt.IsZero() || (a.Status != "done" && a.Status != "error") {
		return false
	}
	return sub.LastActivity.After(a.EndedAt.Add(wokenSlack))
}

// matchSub finds a launch's own transcript: by agent id, else by its prompt
// among transcripts no other launch has claimed.
func matchSub(a *AgentLaunch, subs map[string]*Transcript, used map[string]bool) *Transcript {
	if s := subs[a.AgentID]; a.AgentID != "" && s != nil {
		return s
	}
	if a.prompt == "" {
		return nil
	}
	want := textsafe.Clip(textsafe.Redact(a.prompt), textCap)
	for id, s := range subs {
		if !used[id] && s.FirstPrompt == want {
			return s
		}
	}
	return nil
}

// readSubMeta fills the description Claude Code records beside a subagent's
// transcript (agent-<id>.meta.json). It names an agent whose launch, and so
// whose prompt, fell outside what the transcript reader holds.
func readSubMeta(jsonlPath string, s *Transcript) {
	b, err := os.ReadFile(strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json")
	if err != nil || len(b) > 64<<10 {
		return
	}
	var m struct {
		Description string `json:"description"`
	}
	if json.Unmarshal(b, &m) == nil {
		s.MetaDesc = textsafe.Clip(textsafe.Redact(m.Description), 80)
	}
}
