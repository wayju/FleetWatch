package claudecode

// Parked subagents. A background agent that stops while its own
// background work (a CI watch, say) is still running gets a <task-notification>
// with status "completed" and the summary `Agent "…" is waiting on its own
// background work`, and Claude Code writes finished:true into its meta file.
// Neither means the agent is done: it resumes when that work reports back. The
// agent is "parked" until a different notification names it, it hands back, or
// its own transcript grows past the parked notification.

import (
	"fmt"
	"github.com/wayju/fleetwatch/internal/board"
	"regexp"
	"strings"
	"time"
)

const parkedSuffix = "is waiting on its own background work"

// ParkStale ends a park that nothing has moved for this long. Some background
// work never reports back (a dev server an agent left running), and an agent
// parked on it would otherwise hold its session at WAIT(C) indefinitely.
const ParkStale = time.Hour

var noteSummaryRe = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)

// parkInfo is what the parked notification said and when.
type parkInfo struct {
	summary string
	at      time.Time
}

func noteSummary(txt string) string {
	if m := noteSummaryRe.FindStringSubmatch(txt); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// noteAgents applies a harness notification to the agents it names: running or
// parked agents are parked by a parked summary and ended by any other.
func (t *Transcript) noteAgents(txt string, at time.Time) {
	summary := noteSummary(txt)
	parked := strings.HasSuffix(summary, parkedSuffix)
	for _, a := range t.Agents {
		if (a.Status != "running" && a.Status != "parked") || a.AgentID == "" || !strings.Contains(txt, a.AgentID) {
			continue
		}
		if parked {
			a.Status, a.parked = "parked", &parkInfo{summary: summary, at: at}
			continue
		}
		a.parked = nil
		a.finish(noteStatus(txt), at)
	}
}

// unparkedBy reports whether the agent's own transcript has moved since it
// parked, so the agent is working again.
func (a *AgentLaunch) unparkedBy(sub *Transcript) bool {
	return a.parked != nil && sub != nil && sub.LastActivity.After(a.parked.at)
}

// parkView settles a parked agent's status for the view at now: running again
// once its transcript has moved, done (as of the park) once ParkStale passes.
func (a *AgentLaunch) parkView(now time.Time, sub *Transcript, av *board.AgentView) {
	switch {
	case a.unparkedBy(sub):
		av.Status = "running"
	case a.parked != nil && now.Sub(a.parked.at) > ParkStale:
		av.Status, av.EndedAt = "done", board.Unix(a.parked.at)
	}
}

// parkedCount is how many agents the view still shows parked.
func parkedCount(agents []board.AgentView) int {
	n := 0
	for _, a := range agents {
		if a.Status == "parked" {
			n++
		}
	}
	return n
}

// conditionLabel names the pending conditions: background tasks (with the age
// of a long-lived shell, when given) and parked agents.
func conditionLabel(tasks, parked int, age string) string {
	var parts []string
	if tasks == 1 {
		parts = append(parts, "1 background task"+withAge(age))
	} else if tasks > 1 {
		parts = append(parts, fmt.Sprintf("%d background tasks", tasks)+withAge(age))
	}
	if parked == 1 {
		parts = append(parts, "1 agent waiting on background work")
	} else if parked > 1 {
		parts = append(parts, fmt.Sprintf("%d agents waiting on background work", parked))
	}
	return strings.Join(parts, ", ")
}

func withAge(age string) string {
	if age == "" {
		return ""
	}
	return " (" + age + ")"
}
