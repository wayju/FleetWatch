package core

// The second half of a pass: turn bound sessions into views and the views into
// a Snapshot.

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
)

// Collect runs one full pass.
func (c *Collector) Collect(now time.Time) Snapshot {
	claimed := map[string]bool{}
	links := c.bindExact(c.rts.processes(now, c.claim), claimed)
	c.observe(now, links, claimed)
	for _, a := range c.agents {
		a.Tail(now, claimed)
	}
	c.bindGuess(links, claimed)
	return c.assemble(now, links)
}

// sessionFor is the agent holding a link's session and the session itself;
// the session is nil until a transcript is bound. A process no agent holds
// still reports through the agent that claimed it.
func (c *Collector) sessionFor(l *liveLink) (agent.Agent, agent.Session) {
	if l.sid != "" {
		for _, a := range c.agents {
			if s := a.Session(l.sid); s != nil {
				return a, s
			}
		}
	}
	return c.agentNamed(l.proc.Agent), nil
}

// assemble builds the views for live sessions and recently ended ones, then
// the board-level derivations over them.
func (c *Collector) assemble(now time.Time, links []*liveLink) Snapshot {
	snap := Snapshot{GeneratedAt: now.Unix(), Host: c.cfg.Host}
	c.jobs = c.jobs[:0]
	srcs := map[string]agent.Session{}
	linked := map[string]bool{}
	for _, l := range links {
		a, s := c.sessionFor(l)
		if l.sid != "" {
			linked[l.sid] = true
		}
		snap.Sessions = append(snap.Sessions, c.view(now, l, a, s))
		if s != nil {
			srcs[l.sid] = s
		}
	}
	c.addEnded(&snap, now, linked, srcs)
	for _, a := range c.agents {
		a.EndPass(now)
	}
	c.finishBoard(&snap, now, srcs)
	return snap
}

// addEnded adds the sessions that have a transcript but no live process, for
// as long as they stay inside Recent.
func (c *Collector) addEnded(snap *Snapshot, now time.Time, linked map[string]bool, srcs map[string]agent.Session) {
	for _, a := range c.agents {
		for _, id := range a.Recent(now, c.cfg.Recent, linked) {
			s := a.Session(id)
			l := &liveLink{proc: Proc{Agent: a.Name(), Slot: s.Label(), ConfigDir: s.ConfigDir()}, sid: id, how: "transcript"}
			snap.Sessions = append(snap.Sessions, c.view(now, l, a, s))
			srcs[id] = s
		}
	}
}

// finishBoard derives the board-level views from the sessions.
func (c *Collector) finishBoard(snap *Snapshot, now time.Time, srcs map[string]agent.Session) {
	finish(snap, srcs, c.cfg.ShowText)
	snap.Cost = board.TotalCost(snap.Sessions, c.pricedAsOf)
	snap.Worktrees = c.wt.view(now, snap.Sessions, srcs)
	c.attachServers(snap, now)
	if !c.cfg.ShowText { // repo_path is a host path, like the tool targets text() blanks
		for i := range snap.Worktrees {
			snap.Worktrees[i].RepoPath = ""
		}
	}
	c.recap.Offer(now, c.jobs)
}
