package herdr

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// seenSeq remembers when a pane's state last changed, so the board's
// state_since does not reset on every poll.
type seenSeq struct {
	seq    uint64
	status string
	since  time.Time // zero until a change is observed; the first sight is not a change
}

type explainKey struct {
	pane string
	seq  uint64
}

// signal turns herdr's reading of one pane into a state signal. idle and done
// both mean the agent is ready for input, which the board calls waiting (it
// demotes to idle after half an hour, as for hook states). unknown makes no
// claim.
func (r *Runtime) signal(a agentInfo, now time.Time) (host.StateSignal, bool) {
	var state, reason string
	switch a.Status {
	case "working":
		state = "working"
	case "idle", "done":
		state = "waiting"
	case "blocked":
		state, reason = r.blocked(a)
	default:
		return host.StateSignal{}, false
	}
	rank := r.cfg.Rank
	if a.ScreenSkipped {
		// herdr stopped reading the screen because the agent's own hooks report
		// to it: the same authority as a native state file.
		rank = host.RankNative
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	prev, ok := r.seen[a.PaneID]
	cur := seenSeq{seq: a.Seq, status: a.Status, since: prev.since}
	if ok && (prev.seq != a.Seq || prev.status != a.Status) {
		cur.since = now
	}
	r.seen[a.PaneID] = cur
	return host.StateSignal{State: state, Reason: reason, Source: "herdr", Rank: rank, ObservedAt: cur.since}, true
}

// prune forgets panes that are gone. The caller holds mu.
func (r *Runtime) prune(live map[string]bool) {
	for p := range r.seen {
		if !live[p] {
			delete(r.seen, p)
		}
	}
	for k := range r.explain {
		if !live[k.pane] {
			delete(r.explain, k)
		}
	}
}

// blocked tells an approval prompt from a question. herdr's blocked covers
// approval, question and permission UI; agent.explain names the matched rule,
// asked once per change of state.
func (r *Runtime) blocked(a agentInfo) (state, reason string) {
	k := explainKey{a.PaneID, a.Seq}
	r.mu.Lock()
	cls, ok := r.explain[k]
	r.mu.Unlock()
	if !ok {
		var res struct {
			Explain json.RawMessage `json:"explain"`
		}
		cls = "approval"
		if err := r.once("agent.explain", map[string]string{"target": a.PaneID}, &res); err == nil {
			cls = classifyExplain(res.Explain)
		}
		r.mu.Lock()
		r.explain[k] = cls
		r.mu.Unlock()
	}
	return cls, "herdr blocked"
}

// classifyExplain reads the matched rule out of an explain object. The schema
// does not type it, so this looks at the string values under any key naming
// the match, and anything it cannot place is an approval, the safe reading
// for a blocked agent.
func classifyExplain(raw json.RawMessage) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return "approval"
	}
	for k, v := range m {
		if !strings.Contains(strings.ToLower(k), "matched") {
			continue
		}
		s := strings.ToLower(flatten(v))
		switch {
		case strings.Contains(s, "question") || strings.Contains(s, "choice"):
			return "question"
		case strings.Contains(s, "approv") || strings.Contains(s, "permission"):
			return "approval"
		}
	}
	return "approval"
}

// flatten joins every string in a JSON value.
func flatten(raw json.RawMessage) string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return ""
	}
	var sb strings.Builder
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			sb.WriteString(t + " ")
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(v)
	return sb.String()
}
