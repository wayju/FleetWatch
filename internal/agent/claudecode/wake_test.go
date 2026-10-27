package claudecode

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
)

func ident(s string) string { return s }

func wakeOf(body, state string, agents int) *board.NextWake {
	tr := NewTranscript("s1")
	applyAll(tr, body)
	return NextWake(tr, state, agents, 0, nil, ident)
}

func wantTimer(t *testing.T, w *board.NextWake, sec int) {
	t.Helper()
	if w == nil || w.Kind != "timer" || w.At != t0.Add(time.Duration(sec)*time.Second).Unix() {
		t.Fatalf("want timer at +%ds, got %+v", sec, w)
	}
}

func TestTimerFromScheduleWakeup(t *testing.T) {
	w := wakeOf(userText(0, "go")+wakeCall(2, "w1", map[string]any{"delaySeconds": 1800, "reason": "check CI"})+assistantEnd(4, "ok"), "waiting", 0)
	wantTimer(t, w, 1802)
	if w.Reason != "check CI" {
		t.Fatalf("reason %q", w.Reason)
	}
}

func TestTimerDelayClampedWithoutReceipt(t *testing.T) {
	short := assistantTool(0, "w1", "ScheduleWakeup", map[string]any{"delaySeconds": 5, "reason": "r"})
	wantTimer(t, wakeOf(short, "idle", 0), 60)
	long := assistantTool(0, "w1", "ScheduleWakeup", map[string]any{"delaySeconds": 99999, "reason": "r"})
	wantTimer(t, wakeOf(long, "idle", 0), 3600)
}

func TestLaterScheduleWakeupSupersedes(t *testing.T) {
	body := wakeCall(0, "w1", map[string]any{"delaySeconds": 1800, "reason": "first"}) +
		wakeCall(10, "w2", map[string]any{"delaySeconds": 600, "reason": "second"})
	w := wakeOf(body, "waiting", 0)
	wantTimer(t, w, 610)
	if w.Reason != "second" {
		t.Fatalf("reason %q", w.Reason)
	}
}

func TestStopCancelsTimer(t *testing.T) {
	body := wakeCall(0, "w1", map[string]any{"delaySeconds": 1800, "reason": "x"}) +
		assistantTool(10, "w2", "ScheduleWakeup", map[string]any{"stop": true})
	if w := wakeOf(body, "waiting", 0); w != nil {
		t.Fatalf("stopped timer still shown: %+v", w)
	}
}

func TestTurnAfterFireConsumesTimer(t *testing.T) {
	timer := wakeCall(0, "w1", map[string]any{"delaySeconds": 60, "reason": "x"})
	// A turn before the fire time leaves it pending; one after consumes it.
	wantTimer(t, wakeOf(timer+userText(30, "hello")+assistantEnd(40, "hi"), "waiting", 0), 60)
	if w := wakeOf(timer+userText(65, "wake")+assistantEnd(70, "done"), "waiting", 0); w != nil {
		t.Fatalf("consumed timer still shown: %+v", w)
	}
	// ...and a timer set in the new turn applies again.
	body := timer + userText(65, "wake") + wakeCall(70, "w2", map[string]any{"delaySeconds": 300, "reason": "again"})
	wantTimer(t, wakeOf(body, "waiting", 0), 370)
}

// Mid-turn activity after the fire time (assistant lines, tool results, task
// notifications) is the wake still arriving, not the wake having been taken.
func TestMidTurnLinesDoNotConsumeTimer(t *testing.T) {
	timer := wakeCall(0, "w1", map[string]any{"delaySeconds": 60, "reason": "x", "prompt": "check the build"})
	mid := assistantTool(70, "t1", "Bash", map[string]any{"command": "ls"}) + toolResult(71, "t1", "ok", nil) +
		notification(72, "other", "zzz") + assistantEnd(73, "still going")
	wantTimer(t, wakeOf(timer+mid, "waiting", 0), 60)
}

func TestWakePromptOrRealPromptConsumes(t *testing.T) {
	timer := wakeCall(0, "w1", map[string]any{"delaySeconds": 60, "reason": "x", "prompt": "<<autonomous-loop-dynamic>>"})
	if w := wakeOf(timer+userText(70, "<<autonomous-loop-dynamic>>"), "waiting", 0); w != nil {
		t.Fatalf("sentinel prompt did not consume: %+v", w)
	}
	meta := line(map[string]any{"type": "user", "timestamp": ts(70), "isMeta": true,
		"message": map[string]any{"role": "user", "content": "<system-reminder>check the build now"}})
	custom := wakeCall(0, "w1", map[string]any{"delaySeconds": 60, "reason": "x", "prompt": "<system-reminder>check the build"})
	if w := wakeOf(custom+meta, "waiting", 0); w != nil {
		t.Fatalf("injected wake prompt did not consume: %+v", w)
	}
	other := line(map[string]any{"type": "user", "timestamp": ts(70), "isMeta": true,
		"message": map[string]any{"role": "user", "content": "<system-reminder>unrelated"}})
	wantTimer(t, wakeOf(custom+other, "waiting", 0), 60)
}

// A <<sentinel>> prompt is replaced by the runtime with resolved instructions
// (an injected isMeta line), so the sentinel never prefixes the injected text.
func TestSentinelWakeConsumedByInjectedLine(t *testing.T) {
	timer := wakeCall(0, "w1", map[string]any{"delaySeconds": 60, "reason": "x", "prompt": "<<autonomous-loop-dynamic>>"})
	meta := line(map[string]any{"type": "user", "timestamp": ts(70), "isMeta": true,
		"message": map[string]any{"role": "user", "content": "Continue the autonomous loop: check the build."}})
	if w := wakeOf(timer+meta, "waiting", 0); w != nil {
		t.Fatalf("resolved sentinel wake did not consume: %+v", w)
	}
	wantTimer(t, wakeOf(timer+notification(70, "other", "zzz"), "waiting", 0), 60)
}

func TestBackgroundBashIsConditionUntilNotified(t *testing.T) {
	body := userText(0, "go") + bgBash(1, "b1", "tid1") + assistantEnd(5, "waiting")
	w := wakeOf(body, "waiting", 0)
	if w == nil || w.Kind != "condition" || w.At != 0 || w.Reason != "1 background task" {
		t.Fatalf("got %+v", w)
	}
	if w := wakeOf(body+notification(60, "tid1", "b1"), "waiting", 0); w != nil {
		t.Fatalf("notified task still pending: %+v", w)
	}
}

func TestNotificationMatchesByToolUseIDOrTaskID(t *testing.T) {
	two := bgBash(0, "b1", "tid1") + monitor(2, "m1", "tid2")
	if w := wakeOf(two, "idle", 0); w == nil || w.Reason != "2 background tasks" {
		t.Fatalf("got %+v", w)
	}
	if w := wakeOf(two+notification(9, "tid2", ""), "idle", 0); w == nil || w.Reason != "1 background task" {
		t.Fatalf("monitor by task id: %+v", w)
	}
	if w := wakeOf(two+notification(9, "other", "b1")+notification(10, "tid2", "zzz"), "idle", 0); w != nil {
		t.Fatalf("both cleared: %+v", w)
	}
}

func TestTaskStopAndFailedLaunchClearCondition(t *testing.T) {
	stopped := monitor(0, "m1", "tid2") + assistantTool(5, "s1", "TaskStop", map[string]any{"task_id": "tid2"})
	if w := wakeOf(stopped, "waiting", 0); w != nil {
		t.Fatalf("stopped monitor: %+v", w)
	}
	failed := assistantTool(0, "b1", "Bash", map[string]any{"command": "x", "run_in_background": true}) +
		line(map[string]any{"type": "user", "timestamp": ts(1), "message": map[string]any{"role": "user",
			"content": []any{map[string]any{"type": "tool_result", "tool_use_id": "b1", "content": "boom", "is_error": true}}}})
	if w := wakeOf(failed, "waiting", 0); w != nil {
		t.Fatalf("failed launch: %+v", w)
	}
}

func TestForegroundBashIsNotACondition(t *testing.T) {
	body := assistantTool(0, "b1", "Bash", map[string]any{"command": "ls"}) + toolResult(1, "b1", "ok", nil)
	if w := wakeOf(body, "waiting", 0); w != nil {
		t.Fatalf("got %+v", w)
	}
}

func TestAsyncAgentConditionClearsOnHandback(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1"))
	pending := 0
	for _, a := range tr.Agents {
		if a.Background && a.Status == "running" {
			pending++
		}
	}
	if w := NextWake(tr, "waiting", pending, 0, nil, ident); w == nil || w.Kind != "condition" || w.Reason != "1 background task" {
		t.Fatalf("pending agent: %+v", w)
	}
	applyAll(tr, peerLine(30, "ag1", true))
	if tr.Agents[0].Status != "done" {
		t.Fatalf("handback did not finish agent")
	}
	if w := NextWake(tr, "waiting", 0, 0, nil, ident); w != nil {
		t.Fatalf("handed back: %+v", w)
	}
}

func TestTimerWinsOverCondition(t *testing.T) {
	body := bgBash(0, "b1", "tid1") + wakeCall(5, "w1", map[string]any{"delaySeconds": 900, "reason": "poll"})
	wantTimer(t, wakeOf(body, "waiting", 0), 905)
}

func TestOnlyWaitingAndIdleCarryNextWake(t *testing.T) {
	body := bgBash(0, "b1", "tid1") + wakeCall(5, "w1", map[string]any{"delaySeconds": 900, "reason": "poll"})
	for _, st := range []string{"working", "delegating", "approval", "question", "ended", "starting"} {
		if w := wakeOf(body, st, 0); w != nil {
			t.Fatalf("%s must show nothing: %+v", st, w)
		}
	}
	for _, st := range []string{"waiting", "idle"} {
		if w := wakeOf(body, st, 0); w == nil {
			t.Fatalf("%s must show a wake", st)
		}
	}
}

func TestReasonRedactedAndClipped(t *testing.T) {
	body := wakeCall(0, "w1", map[string]any{"delaySeconds": 600, "reason": "poll token=supersecretvalue123 " + strings.Repeat("x", 300)})
	w := wakeOf(body, "waiting", 0)
	if strings.Contains(w.Reason, "supersecretvalue123") || len([]rune(w.Reason)) > wakeReasonCap {
		t.Fatalf("reason %q", w.Reason)
	}
}

func TestBackgroundTasksAreCapped(t *testing.T) {
	var body string
	for i := 0; i < maxBgTasks+30; i++ {
		body += assistantTool(i, fmt.Sprintf("b%d", i), "Bash", map[string]any{"command": "x", "run_in_background": true})
	}
	tr := NewTranscript("s1")
	applyAll(tr, body)
	if len(tr.wake.bg) != maxBgTasks {
		t.Fatalf("bg tasks %d, want cap %d", len(tr.wake.bg), maxBgTasks)
	}
}

func wakeCapped(sh *host.TaskCap, agents int, launches int) *board.NextWake {
	tr := NewTranscript("s1")
	for i := 0; i < launches; i++ {
		applyAll(tr, bgBash(i*2, fmt.Sprintf("b%d", i), fmt.Sprintf("task%d", i)))
	}
	return NextWake(tr, "waiting", agents, 0, sh, ident)
}

func TestLiveShellCountCapsPendingBash(t *testing.T) {
	if w := wakeCapped(nil, 0, 3); w == nil || w.Reason != "3 background tasks" {
		t.Errorf("no scan data keeps current behaviour: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{Count: 1}, 0, 3); w == nil || w.Reason != "1 background task" {
		t.Errorf("capped at one live shell: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{}, 0, 3); w != nil {
		t.Errorf("no live shell and nothing else: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{Count: 5}, 0, 2); w == nil || w.Reason != "2 background tasks" {
		t.Errorf("cap never raises the count: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{}, 1, 3); w == nil || w.Reason != "1 background task" {
		t.Errorf("async agents are unaffected by the cap: %+v", w)
	}
}

func TestOldLiveShellAgeAppendedToReason(t *testing.T) {
	if w := wakeCapped(&host.TaskCap{Count: 1, Age: 34 * time.Hour}, 0, 1); w == nil || w.Reason != "1 background task (1d 10h)" {
		t.Errorf("1d 10h: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{Count: 1, Age: 7*time.Hour + 20*time.Minute}, 0, 1); w == nil || w.Reason != "1 background task (7h)" {
		t.Errorf("7h: %+v", w)
	}
	if w := wakeCapped(&host.TaskCap{Count: 1, Age: 5*time.Hour + 59*time.Minute}, 0, 1); w == nil || w.Reason != "1 background task" {
		t.Errorf("under 6h is a normal wait: %+v", w)
	}
}
