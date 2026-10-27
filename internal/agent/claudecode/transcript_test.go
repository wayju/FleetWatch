package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"bytes"
)

func TestTranscriptFold(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "please fix #42 and see https://git.example.test/acme/app/issues/7")+
		line(map[string]any{"type": "user", "isMeta": true, "timestamp": ts(1), "message": map[string]any{"role": "user", "content": "<system-reminder>x</system-reminder>"}})+
		assistantTool(2, "t1", "Bash", map[string]any{"command": "git status\nmore"})+
		toolResult(3, "t1", "clean", nil)+
		assistantTool(4, "t2", "Agent", map[string]any{"subagent_type": "Explore", "description": "map repo", "prompt": "find things", "model": "haiku"})+
		toolResult(5, "t2", "done.\nagentId: a1b2c3", nil)+
		assistantTool(6, "t3", "SendMessage", map[string]any{"to": "otherses", "message": "token=supersecretvalue123 hi"})+
		assistantEnd(7, "All done."))

	if tr.Cwd != "/repo" || tr.Branch != "feature/42-fleet" || tr.ClaudeVersion != "2.1.3" || tr.Model != "claude-sonnet-5-5" {
		t.Fatalf("metadata: %+v", tr)
	}
	if !strings.HasPrefix(tr.FirstPrompt, "please fix #42") || tr.LastPrompt != tr.FirstPrompt {
		t.Fatalf("meta line must not count as a prompt: %q / %q", tr.FirstPrompt, tr.LastPrompt)
	}
	if tr.LastKind != "end_turn" || tr.LastReply != "All done." {
		t.Fatalf("last kind %q reply %q", tr.LastKind, tr.LastReply)
	}
	if tr.ContextTokens != 50000 {
		t.Fatalf("context %d", tr.ContextTokens)
	}
	if tr.Tools[0].Target != "git status …" || tr.Tools[0].Status != "ok" {
		t.Fatalf("tool: %+v", tr.Tools[0])
	}
	if len(tr.Agents) != 1 || tr.Agents[0].AgentID != "a1b2c3" || tr.Agents[0].Status != "done" || tr.Agents[0].Type != "Explore" {
		t.Fatalf("agent: %+v", tr.Agents)
	}
	if len(tr.Messages) != 1 || tr.Messages[0].To != "otherses" || strings.Contains(tr.Messages[0].Preview, "supersecret") {
		t.Fatalf("message: %+v", tr.Messages)
	}
	if tr.PendingTools() != 1 { // the SendMessage call has no result yet
		t.Fatalf("pending %d", tr.PendingTools())
	}
}

func TestBackgroundAgentFinishesOnNotification(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, assistantTool(0, "t1", "Task", map[string]any{"description": "audit", "prompt": "p", "run_in_background": true})+
		toolResult(1, "t1", "Async agent launched", map[string]any{"agentId": "bg1"}))
	if a := tr.Agents[0]; a.Status != "running" || a.AgentID != "bg1" {
		t.Fatalf("background receipt must not finish it: %+v", a)
	}
	applyAll(tr, line(map[string]any{"type": "user", "timestamp": ts(9), "message": map[string]any{"role": "user",
		"content": "<task-notification><task-id>bg1</task-id><status>completed</status></task-notification>"}}))
	if a := tr.Agents[0]; a.Status != "done" {
		t.Fatalf("notification should finish it: %+v", a)
	}
	if tr.FirstPrompt != "" {
		t.Fatalf("notification is not a prompt: %q", tr.FirstPrompt)
	}
}

// Claude Code launches agents asynchronously by default: no run_in_background
// in the input, only isAsync/async_launched on the receipt.
func TestDefaultAsyncAgentStaysRunningAfterReceipt(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, assistantTool(0, "t1", "Agent", map[string]any{"subagent_type": "code-reviewer", "description": "review", "prompt": "p"})+
		toolResult(1, "t1", "Async agent launched successfully.", map[string]any{"agentId": "as1", "isAsync": true, "status": "async_launched"}))
	if a := tr.Agents[0]; a.Status != "running" || !a.Background || a.AgentID != "as1" {
		t.Fatalf("async receipt must not finish it: %+v", a)
	}
	applyAll(tr, line(map[string]any{"type": "user", "timestamp": ts(9), "message": map[string]any{"role": "user",
		"content": "<task-notification><task-id>as1</task-id><status>completed</status></task-notification>"}}))
	if a := tr.Agents[0]; a.Status != "done" {
		t.Fatalf("notification should finish it: %+v", a)
	}
}

// A synchronous agent's result is its end.
func TestSyncAgentFinishesOnResult(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, assistantTool(0, "t1", "Agent", map[string]any{"description": "inline", "prompt": "p"})+
		toolResult(1, "t1", "done: found 3 files", map[string]any{"agentId": "sy1", "status": "completed"}))
	if a := tr.Agents[0]; a.Status != "done" || a.Background {
		t.Fatalf("sync result should finish it: %+v", a)
	}
}

func TestSidechainLinesGoToTheirAgent(t *testing.T) {
	tr := NewTranscript("s1")
	tr.Apply([]byte(`{"type":"assistant","isSidechain":true,"agentId":"ag9","timestamp":"` + ts(1) + `","message":{"role":"assistant","content":[{"type":"tool_use","id":"x","name":"Grep","input":{"pattern":"foo"}}]}}`))
	if len(tr.Tools) != 0 || tr.Sidechains["ag9"] == nil || tr.Sidechains["ag9"].Tools[0].Target != "foo" {
		t.Fatalf("sidechain routing: %+v %+v", tr.Tools, tr.Sidechains)
	}
}

func TestTailIsIncrementalAndHandlesPartialLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "s.jsonl")
	full := userText(0, "first") + assistantEnd(1, "reply")
	cut := len(userText(0, "first")) + 10
	if err := os.WriteFile(p, []byte(full[:cut]), 0o644); err != nil {
		t.Fatal(err)
	}
	tl := newTail(p, "s")
	if err := tl.poll(t0); err != nil {
		t.Fatal(err)
	}
	if tl.T.FirstPrompt != "first" || tl.T.LastReply != "" {
		t.Fatalf("after partial: %+v", tl.T)
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(full[cut:])
	f.Close()
	_ = tl.poll(t0)
	if tl.T.LastReply != "reply" {
		t.Fatalf("partial line not completed: %q", tl.T.LastReply)
	}
	// Truncation restarts from scratch.
	os.WriteFile(p, []byte(userText(0, "again")), 0o644)
	_ = tl.poll(t0)
	if tl.T.FirstPrompt != "again" || tl.T.LastReply != "" {
		t.Fatalf("truncate not reset: %+v", tl.T)
	}
}

func systemLine(sec int, subtype, content string) string {
	return line(map[string]any{"type": "system", "subtype": subtype, "content": content, "timestamp": ts(sec)})
}

// An idle compaction after a finished turn leaves the session waiting.
func TestCompactionLinesDoNotChangeState(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "go")+assistantEnd(10, "done")+
		systemLine(600, "compact_boundary", "Conversation compacted")+
		line(map[string]any{"type": "user", "isCompactSummary": true, "timestamp": ts(601),
			"message": map[string]any{"role": "user", "content": "This session is being continued from a previous conversation..."}})+
		systemLine(602, "informational", "Compacted while idle, before the prompt cache expired"))
	if tr.LastKind != "end_turn" || !tr.LastActivity.Equal(t0.Add(10*time.Second)) {
		t.Fatalf("kind %q activity %v", tr.LastKind, tr.LastActivity)
	}
	if got := TranscriptState(t0.Add(10*time.Minute), tr); got != "waiting" {
		t.Fatalf("state %s", got)
	}
	if tr.LastPrompt != "go" {
		t.Fatalf("summary became the last prompt: %q", tr.LastPrompt)
	}
}

func TestTailBoundsMemoryOnLargeTranscript(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.jsonl")
	var b bytes.Buffer
	b.WriteString(userText(0, "ancient first prompt"))
	filler := assistantEnd(1, strings.Repeat("x", 1000))
	for b.Len() < 20<<20 {
		b.WriteString(filler)
	}
	// One line over the cap, then a normal tail.
	b.WriteString(`{"type":"user","pad":"` + strings.Repeat("y", maxLine+10) + "\"}\n")
	b.WriteString(userText(5, "recent prompt"))
	b.WriteString(assistantEnd(6, "final reply"))
	if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	tl := newTail(p, "s")
	for i := 0; i < 3; i++ {
		if err := tl.poll(t0); err != nil {
			t.Fatal(err)
		}
	}
	if tl.offset != int64(b.Len()) {
		t.Fatalf("offset %d, want %d", tl.offset, b.Len())
	}
	if tl.T.FirstPrompt == "ancient first prompt" {
		t.Error("a first open must start near the end of a huge transcript")
	}
	if tl.T.LastPrompt != "recent prompt" || tl.T.LastReply != "final reply" {
		t.Errorf("lost the lines after the oversized one: %q %q", tl.T.LastPrompt, tl.T.LastReply)
	}
	if len(tl.partial) > maxLine {
		t.Errorf("partial %d over cap", len(tl.partial))
	}

	// A partial line longer than the cap is dropped, and reading resumes after it.
	tl2 := newTail(filepath.Join(t.TempDir(), "p.jsonl"), "s")
	if err := os.WriteFile(tl2.path, []byte(userText(0, "one")+strings.Repeat("z", maxLine+5)), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = tl2.poll(t0)
	if len(tl2.partial) > maxLine || !tl2.skipping {
		t.Errorf("partial %d skipping %v", len(tl2.partial), tl2.skipping)
	}
	f, _ := os.OpenFile(tl2.path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("\n" + assistantEnd(2, "after"))
	f.Close()
	_ = tl2.poll(t0)
	if tl2.T.LastReply != "after" {
		t.Errorf("did not resume after the long line: %q", tl2.T.LastReply)
	}
}

func TestTailRefusesFIFOAndSymlink(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "f.jsonl")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- newTail(fifo, "f").poll(t0) }()
	select {
	case err := <-done:
		if err == nil {
			t.Error("a FIFO must be refused")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("poll blocked on a FIFO")
	}
	real := filepath.Join(dir, "real.jsonl")
	os.WriteFile(real, []byte(userText(0, "hi")), 0o644)
	link := filepath.Join(dir, "link.jsonl")
	os.Symlink(real, link)
	if err := newTail(link, "l").poll(t0); err == nil {
		t.Error("a symlink must be refused")
	}
}

func TestRepeatedToolUseLineDedupes(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, sendMsg(1, "tu1", "x", "hi")+sendMsg(2, "tu1", "x", "hi")+sendMsg(3, "tu2", "x", "hi"))
	if len(tr.Messages) != 2 {
		t.Fatalf("%d messages", len(tr.Messages))
	}
}

func TestRedactBeforeClipAtTheCap(t *testing.T) {
	secret := "sk-" + strings.Repeat("A1b2C3d4", 5)
	// The secret straddles textCap: clipping first would leave an unmatched
	// "sk-A1b2…" fragment, redacting first removes all of it.
	reply := strings.Repeat("x", textCap-8) + " " + secret + " tail"
	tr := NewTranscript("s1")
	applyAll(tr, assistantEnd(1, reply))
	if strings.Contains(tr.LastReply, "sk-") || strings.Contains(tr.LastReply, "A1b2") {
		t.Fatalf("secret fragment survived the clip: %q", tr.LastReply)
	}
	prompt := strings.Repeat("y", 400-8) + " " + secret
	tr = NewTranscript("s2")
	applyAll(tr, assistantTool(1, "a1", "Agent", map[string]any{"prompt": prompt}))
	if len(tr.Agents) != 1 || strings.Contains(tr.Agents[0].prompt, "A1b2") {
		t.Fatalf("agent prompt kept a secret fragment: %+v", tr.Agents)
	}
}

func TestMessageLabelFollowsShowText(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, assistantTool(1, "m1", "SendMessage", map[string]any{"to": "token=hunter2hunter2", "message": "hi"}))
	var ev Event
	for _, e := range tr.Events {
		if e.Kind == "message" {
			ev = e
		}
	}
	if got := eventText(ev, false); got != "→ message" {
		t.Errorf("text off: %q", got)
	}
	if got := eventText(Event{Kind: "message", Text: "→ token=hunter2hunter2"}, true); strings.Contains(got, "hunter2") {
		t.Errorf("label not redacted with text on: %q", got)
	}
}
