package core

// The first half of a pass: discover what is running, bind each process to a
// session (exact proof first, a guess only for what nothing else could place).
// Collect in collect_assemble.go calls these in order.

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/host"
)

type liveLink struct {
	proc Proc
	sid  string
	peer string
	how  string
	pane string
	obs  host.Observation // what the runtimes know about this process
}

// claim asks the agents, in config order, whether a process is theirs.
func (c *Collector) claim(comm string, cmdline func() string) (string, bool) {
	for _, a := range c.agents {
		if a.Claims(comm, cmdline) {
			return a.Name(), true
		}
	}
	return "", false
}

func (c *Collector) agentNamed(name string) agent.Agent {
	for _, a := range c.agents {
		if a.Name() == name {
			return a
		}
	}
	return nil
}

// bindExact links each live process to the session its agent can prove.
// Processes are visited newest first.
func (c *Collector) bindExact(procs []Proc, claimed map[string]bool) []*liveLink {
	var links []*liveLink
	for _, p := range procs {
		l := &liveLink{proc: p, pane: p.Address}
		if a := c.agentNamed(p.Agent); a != nil {
			if b, ok := a.Bind(p.session(), agent.BindExact, claimed); ok {
				l.peer = b.Peer
				if b.SessionID != "" {
					l.sid, l.how = b.SessionID, b.How
					claimed[b.SessionID] = true
				}
			}
		}
		links = append(links, l)
	}
	return links
}

// observe lets the runtimes place and describe every bound process. A process
// the agent did not place takes a runtime's session-id hint (the HUD
// projection) before any guess is made.
func (c *Collector) observe(now time.Time, links []*liveLink, claimed map[string]bool) {
	bs := make([]host.Bound, len(links))
	for i, l := range links {
		bs[i] = host.Bound{Session: l.proc.session(), SessionID: l.sid}
	}
	c.servers = map[int][]ServerView{}
	for i, o := range c.rts.observe(now, bs) {
		l := links[i]
		l.obs = o
		if l.sid == "" && o.SessionID != "" {
			l.sid, l.how = o.SessionID, o.How
			claimed[l.sid] = true
		}
		if o.Address != "" {
			l.pane = o.Address
		}
		if len(o.Servers) > 0 {
			c.servers[l.proc.PID] = o.Servers
		}
	}
}

// bindGuess places the processes nothing else could, in the same order the
// exact binding visited them; each guess is claimed before the next.
func (c *Collector) bindGuess(links []*liveLink, claimed map[string]bool) {
	for _, l := range links {
		a := c.agentNamed(l.proc.Agent)
		if l.sid != "" || a == nil {
			continue
		}
		if b, ok := a.Bind(l.proc.session(), agent.BindGuess, claimed); ok {
			l.sid, l.how = b.SessionID, b.How
			claimed[l.sid] = true
		}
	}
}
