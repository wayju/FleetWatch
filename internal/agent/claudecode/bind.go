package claudecode

// Placing a process on a transcript.

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/host"
)

// Bind places a process. Exact binding trusts only the session file Claude
// Code writes for each process (see Continued for a session that has moved on);
// a guess takes the newest unclaimed transcript
// that was active since the process started, in the same config-dir label.
func (a *Agent) Bind(s host.Session, mode agent.BindMode, claimed map[string]bool) (agent.Binding, bool) {
	if mode == agent.BindExact {
		dir := s.ConfigDir
		if dir == "" {
			dir = a.defaultDir()
		}
		sid, peer := sessionFileInfo(dir, s.PID)
		if sid == "" && peer == "" {
			return agent.Binding{}, false
		}
		b := agent.Binding{SessionID: sid, Peer: peer}
		if sid != "" {
			b.How = "session-file"
		}
		return b, true
	}
	var best *tail
	for _, tl := range a.tails {
		id := tl.T.ID
		if claimed[id] || tl.T.LastActivity.Before(s.Start.Add(-time.Minute)) {
			continue
		}
		if ref := a.index[id]; s.Label != "" && ref.label != "" && ref.label != s.Label {
			continue
		}
		// Equal activity: the smaller id, so map order cannot decide.
		if best == nil || tl.T.LastActivity.After(best.T.LastActivity) ||
			(tl.T.LastActivity.Equal(best.T.LastActivity) && id < best.T.ID) {
			best = tl
		}
	}
	if best == nil {
		return agent.Binding{}, false
	}
	return agent.Binding{SessionID: best.T.ID, How: "guess"}, true
}
