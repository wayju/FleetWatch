package claudecode

// next_wake: when a waiting session will act again without a human.
//
// Two things in a transcript schedule that. A timer is a ScheduleWakeup call:
// the session resumes at the call's time plus its delay (clamped to 60..3600s).
// A later ScheduleWakeup replaces it, `stop: true` cancels it, and the wake-up prompt
// or a real user prompt after the fire time consumes it. A condition is a background
// Bash, a Monitor or an async agent that has not yet reported back: its
// <task-notification> (or the agent's hand-back) wakes the session.
//
// The timer is the definite time, so it wins over a condition. Only waiting and
// idle sessions carry a next_wake; a delegating session is already woken by its
// agents and the other states are not waiting on anything.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"regexp"
	"strings"
	"time"
)

const (
	minWakeDelay  = 60 * time.Second
	maxWakeDelay  = 3600 * time.Second
	wakeReasonCap = 120
	maxBgTasks    = 64
)

// bgTask is a background Bash or Monitor launch awaiting its notification.
type bgTask struct {
	toolID string
	taskID string // from the launch receipt; empty until it arrives
}

type wakeState struct {
	timerAt     time.Time
	timerReason string
	timerTool   string // the ScheduleWakeup tool_use whose receipt may refine timerAt
	timerPrompt string // the prompt the wake-up injects as a user message
	timerAny    bool   // sentinel prompt: any injected non-notification line consumes
	bg          []*bgTask
}

var (
	noteTaskRe = regexp.MustCompile(`<task-id>\s*([A-Za-z0-9_-]+)\s*</task-id>`)
	noteToolRe = regexp.MustCompile(`<tool-use-id>\s*([A-Za-z0-9_-]+)\s*</tool-use-id>`)
)

// wakeUser consumes a timer when a user line arrives after its fire time that
// is a real prompt or the injected wake-up prompt. Assistant turns, tool
// results and notifications arriving mid-turn do not consume it: the wake is
// still to come. injected is true for harness-injected text, which counts only
// when it is the recorded wake-up prompt.
func (t *Transcript) wakeUser(txt string, injected bool, at time.Time) {
	w := &t.wake
	if w.timerAt.IsZero() || !at.After(w.timerAt) {
		return
	}
	if injected {
		if w.timerAny {
			if strings.HasPrefix(txt, "<task-notification") {
				return
			}
		} else if w.timerPrompt == "" || !strings.HasPrefix(txt, w.timerPrompt) {
			return
		}
	}
	w.timerAt, w.timerReason, w.timerTool, w.timerPrompt, w.timerAny = time.Time{}, "", "", "", false
}

func (t *Transcript) wakeToolUse(b rawBlock, in map[string]any, at time.Time) {
	w := &t.wake
	switch b.Name {
	case "ScheduleWakeup":
		if in["stop"] == true {
			*w = wakeState{bg: w.bg}
			return
		}
		secs, _ := in["delaySeconds"].(float64)
		d := time.Duration(secs) * time.Second
		d = min(max(d, minWakeDelay), maxWakeDelay)
		w.timerAt, w.timerTool = at.Add(d), b.ID
		w.timerReason = textsafe.Clip(textsafe.Redact(str(in, "reason")), wakeReasonCap)
		w.timerPrompt = strings.TrimSpace(str(in, "prompt"))
		// A <<sentinel>> prompt is resolved by the runtime into longer
		// instructions, so the injected text never starts with the sentinel.
		w.timerAny = strings.HasPrefix(w.timerPrompt, "<<") && strings.HasSuffix(w.timerPrompt, ">>")
	case "Monitor":
		w.addBg(b.ID)
	case "Bash":
		if in["run_in_background"] == true {
			w.addBg(b.ID)
		}
	case "TaskStop":
		w.dropBg(str(in, "task_id"), "")
	}
}

// wakeToolResult binds receipts: the scheduled time of a wakeup, the task id of
// a background launch. A failed launch never becomes a condition.
func (t *Transcript) wakeToolResult(b rawBlock, extra json.RawMessage) {
	w := &t.wake
	var m struct {
		ScheduledFor int64  `json:"scheduledFor"` // unix ms
		TaskID       string `json:"taskId"`
		BgTaskID     string `json:"backgroundTaskId"`
	}
	_ = json.Unmarshal(extra, &m)
	if b.ToolUseID == w.timerTool && m.ScheduledFor > 0 {
		w.timerAt = time.UnixMilli(m.ScheduledFor)
	}
	for _, g := range w.bg {
		if g.toolID != b.ToolUseID {
			continue
		}
		if b.IsError {
			w.dropBg("", g.toolID)
			return
		}
		g.taskID = m.TaskID + m.BgTaskID
	}
}

// wakeNotification clears the background task a <task-notification> names.
func (t *Transcript) wakeNotification(txt string) {
	var task, tool string
	if m := noteTaskRe.FindStringSubmatch(txt); m != nil {
		task = m[1]
	}
	if m := noteToolRe.FindStringSubmatch(txt); m != nil {
		tool = m[1]
	}
	t.wake.dropBg(task, tool)
}

// addBg records a launch, dropping the oldest once maxBgTasks is reached so a
// long transcript whose notifications never matched cannot grow without bound.
func (w *wakeState) addBg(toolID string) {
	if len(w.bg) >= maxBgTasks {
		w.bg = append(w.bg[:0], w.bg[1:]...)
	}
	w.bg = append(w.bg, &bgTask{toolID: toolID})
}

func (w *wakeState) dropBg(taskID, toolID string) {
	kept := w.bg[:0]
	for _, g := range w.bg {
		if (taskID != "" && g.taskID == taskID) || (toolID != "" && g.toolID == toolID) {
			continue
		}
		kept = append(kept, g)
	}
	w.bg = kept
}

// NextWake resolves the rule for one session: timer, else condition, else nil.
// asyncAgents counts background agents the view still shows running; parked
// counts those waiting on their own background work. sh, when non-nil,
// is a fresh scan of the live shells: it caps the pending Bash/Monitor count
// and names a shell older than longShell in the reason.
func NextWake(t *Transcript, state string, asyncAgents, parked int, sh *host.TaskCap, text func(string) string) *board.NextWake {
	if state != "waiting" && state != "idle" {
		return nil
	}
	if !t.wake.timerAt.IsZero() {
		return &board.NextWake{Kind: "timer", At: t.wake.timerAt.Unix(), Reason: text(t.wake.timerReason)}
	}
	bg, age := len(t.wake.bg), ""
	if sh != nil {
		bg = min(bg, sh.Count)
		if bg > 0 && sh.Age >= longShell {
			age = ageLabel(sh.Age)
		}
	}
	n := bg + asyncAgents
	if n+parked == 0 {
		return nil
	}
	return &board.NextWake{Kind: "condition", Reason: text(conditionLabel(n, parked, age))}
}
