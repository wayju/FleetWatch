package core

// Per-session view assembly and state resolution.

import (
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"strconv"
	"time"
)

func (c *Collector) view(now time.Time, l *liveLink, ag agent.Agent, sess agent.Session) SessionView {
	live := l.proc.PID != 0
	v := SessionView{
		ID: l.sid, Live: live, Link: l.how, PID: l.proc.PID, PeerName: l.peer, Slot: l.proc.Slot, Pane: l.pane,
		Issues: []IssueRef{}, Tools: []ToolView{}, Agents: []AgentView{}, Messages: []LinkView{},
		Cost: board.EmptyCost(c.pricedAsOf),
	}
	if v.ID == "" {
		v.ID = "pid-" + strconv.Itoa(l.proc.PID)
		v.Link = "none"
	}
	v.Short = board.ShortID(v.ID)
	v.Host = c.cfg.Host
	if live {
		v.Runtime, v.Address, v.Location = l.proc.Runtime, l.pane, l.proc.Location
	}
	if ag != nil {
		v.Account = anonymise(ag.Account(l.proc.ConfigDir), c.cfg.ShowText)
	}
	v.AccountKey = v.Account.Email
	if v.AccountKey == "" {
		v.AccountKey = "unknown"
	}
	if live {
		v.StartedAt = board.Unix(l.proc.Start)
	}
	if sess == nil {
		c.resolveState(&v, l, nil, agent.Detail{}, now)
		return v
	}
	d := sess.Detail(now, c.text)
	c.fillFromDetail(&v, l, d, now)
	c.resolveState(&v, l, sess, d, now)
	return v
}

// fillFromDetail copies what the agent knows about the session into the view
// and adds what the core knows: the repository, its issues, the statusline's
// context figure and the recap.
func (c *Collector) fillFromDetail(v *SessionView, l *liveLink, d agent.Detail, now time.Time) {
	v.Cwd, v.AgentVersion, v.ClaudeVersion, v.Model = d.Cwd, d.Version, d.Version, d.Model
	v.ModelClass = c.cfg.Models.Class(d.Model)
	if v.StartedAt == 0 {
		v.StartedAt = board.Unix(d.StartedAt)
	}
	v.LastActivity = board.Unix(d.LastActivity)
	v.Repo = c.repos.get(d.Cwd, now)
	v.Branch = d.Branch
	if v.Branch == "" || v.Branch == "HEAD" {
		v.Branch = v.Repo.Branch
	}
	if v.Branch != v.Repo.Branch {
		v.Repo.Commit = "" // the checkout has moved on from what the transcript last saw
	}
	v.Cost = d.Cost
	v.Context = d.Context
	c.applyStatusline(v, l, d.LastActivity, now)
	v.Issues = c.issues(now, v.Repo, c.scan.extract(v.Branch, v.Repo.Slug(), d.FirstPrompt, d.LastPrompt))
	// Every free-text field passes through text() once: redacted, and empty
	// when FLEET_SHOW_TEXT=0. The agent has already gated its own.
	text := c.text
	v.Title = text(d.Title)
	for i := range v.Issues {
		v.Issues[i].Title = text(v.Issues[i].Title)
	}
	if c.cfg.ShowText {
		v.Recap = &Recap{FirstPrompt: text(d.FirstPrompt), LastPrompt: text(d.LastPrompt), LastReply: text(d.LastReply)}
		v.Recap.Summary, v.Recap.SummaryAt = c.recap.Get(v.ID)
		v.Recap.Summary = text(v.Recap.Summary)
	}
	v.Tools, v.Current, v.Agents, v.Messages, v.AgentTotals = d.Tools, d.Current, d.Agents, d.Messages, d.AgentTotals
}

// applyStatusline lets the statusline's context figure win over the
// transcript's guess when it is fresh, else a cached figure for the pane.
func (c *Collector) applyStatusline(v *SessionView, l *liveLink, lastActivity, now time.Time) {
	if v.Live && statuslineFresh(now, lastActivity, l.obs.Context) {
		v.Context.Pct, v.Context.Source = float64(l.obs.Context.Pct), "statusline"
		if l.obs.Context.Size > 0 {
			v.Context.Limit = l.obs.Context.Size
		}
	} else if l.pane != "" {
		if pct, ok := c.rts.contextCache(l.pane); ok && v.Live {
			v.Context.Pct, v.Context.Source = pct, "statusline"
		}
	}
}

// text is the one gate for free text on the board: redacted, clipped by the
// producer, and dropped entirely when FLEET_SHOW_TEXT=0.
func (c *Collector) text(s string) string {
	if !c.cfg.ShowText || s == "" {
		return ""
	}
	return textsafe.Redact(s)
}

// ctxFresh is how recently the statusline must have rendered for its figure to
// be trusted outright. An idle session renders nothing, but its fill cannot
// have changed either, so an entry at least as new as the transcript's last
// activity also counts.
const ctxFresh = 10 * time.Minute

func statuslineFresh(now, lastActivity time.Time, e *host.ContextSignal) bool {
	if e == nil || e.Pct <= 0 {
		return false
	}
	at := e.At
	return now.Sub(at) <= ctxFresh || !at.Before(lastActivity.Add(-time.Minute))
}
