package core

// "delegating": the main thread has ended its turn and waits for
// subagents that are still working. pane-state.sh writes nothing to the main
// hook file for a subagent's tool events, so the board would read WAIT or IDLE.
// The state applies when the resolved state is waiting or idle and either an
// agent is running or stalled, or a subagent touched <HookDir>/<pane>.agents.json
// (shape {ts, agent_id, session_id}) within agentsFresh, after the main hook ts.

import (
	"time"
)

const agentsFresh = 90 * time.Second

func (c *Collector) applyDelegating(v *SessionView, l *liveLink, now time.Time) {
	if v.State != "waiting" && v.State != "idle" {
		return
	}
	busy := v.AgentTotals.Running > 0
	for _, a := range v.Agents {
		if a.Status == "running" || a.Status == "stalled" {
			busy = true
		}
	}
	if !busy {
		if at, ok := c.rts.subagentTouched(v.ID, l.pane); ok &&
			at.Unix() > v.StateSince && now.Sub(at) <= agentsFresh {
			busy = true
		}
	}
	if busy {
		v.State = "delegating"
	}
}
