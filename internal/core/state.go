package core

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
)

// mergeState drops signals whose FreshUntil has passed, then takes the highest
// Rank; a tie goes to the newer ObservedAt. since is the winner's ObservedAt,
// or the floor signal's when the winner has none (a projection whose
// observed_at does not parse).
func mergeState(now time.Time, floor host.StateSignal, sigs []host.StateSignal) (host.StateSignal, time.Time) {
	win := floor
	for _, s := range sigs {
		if !s.FreshUntil.IsZero() && now.After(s.FreshUntil) {
			continue
		}
		if s.Rank > win.Rank || (s.Rank == win.Rank && s.ObservedAt.After(win.ObservedAt)) {
			win = s
		}
	}
	since := win.ObservedAt
	if since.IsZero() {
		since = floor.ObservedAt
	}
	return win, since
}

// waitingToIdle is how long a session may wait on its human before the board
// calls it idle, whichever source reported the wait (matches claudecode.TranscriptState).
const waitingToIdle = 30 * time.Minute

// stateFloor is the signal every live session has: the agent's reading of its
// transcript, or "starting" from the process when no session is bound.
func stateFloor(now time.Time, l *liveLink, sess agent.Session) host.StateSignal {
	if sess == nil {
		return host.StateSignal{State: "starting", Source: "process", Rank: host.RankProcess, ObservedAt: l.proc.Start}
	}
	return sess.State(now)
}

// resolveState settles a session's state in a fixed order: not live is ended;
// otherwise the highest-ranked signal wins, waiting for over half an hour is
// idle, and a session with a transcript then gets delegating, its next wake,
// and wake-is-work. A session with no transcript stops after the merge: it has
// no agents or wake to read.
func (c *Collector) resolveState(v *SessionView, l *liveLink, sess agent.Session, d agent.Detail, now time.Time) {
	if !v.Live {
		v.State, v.StateSource, v.StateSince = "ended", "process", v.LastActivity
		return
	}
	win, since := mergeState(now, stateFloor(now, l, sess), l.obs.States)
	v.State, v.StateReason, v.StateSource, v.StateSince = win.State, win.Reason, win.Source, board.Unix(since)
	if v.State == "waiting" && v.StateSince > 0 && now.Sub(time.Unix(v.StateSince, 0)) > waitingToIdle {
		v.State = "idle"
	}
	if sess == nil {
		return
	}
	c.applyDelegating(v, l, now)
	c.applyNextWake(v, sess, l.obs.Tasks)
	wakeIsWork(v)
	if c.cfg.ShowText && c.recap != nil && d.FirstPrompt != "" {
		c.jobs = append(c.jobs, recapJob{id: v.ID, asOf: d.LastActivity, digest: sess.Digest(*v)})
	}
}

// wakeIsWork keeps WAIT for the user's move: a session that will act again on its
// own (a timer or a pending background task) is working, not waiting on him.
// It runs after applyNextWake, which only computes a wake for waiting
// and idle states, and after the 30 minute waiting-to-idle demotion, so a
// demoted session is lifted too. The wake itself stays on the view.
func wakeIsWork(v *SessionView) {
	if (v.State == "waiting" || v.State == "idle") && v.NextWake != nil {
		v.State, v.StateReason = "working", v.NextWake.Reason
	}
}

// applyNextWake asks the agent when the session will act again on its own,
// given the state so far, the background agents the view still shows, and the
// runtime's scan of live shells.
func (c *Collector) applyNextWake(v *SessionView, sess agent.Session, tasks *host.TaskCap) {
	async, parked := 0, 0
	for _, a := range v.Agents {
		if a.Background && (a.Status == "running" || a.Status == "stalled") {
			async++
		}
		if a.Status == "parked" {
			parked++
		}
	}
	v.NextWake = sess.NextWake(v.State, async, parked, tasks, c.text)
}
