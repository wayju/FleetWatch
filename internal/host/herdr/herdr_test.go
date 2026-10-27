package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

var t0 = time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)

func claimClaude(comm string, _ func() string) (string, bool) { return "claudecode", comm == "claude" }

func agentList(agents ...map[string]any) map[string]any {
	return map[string]any{"type": "agent_list", "agents": agents}
}

func info(pane, agent, status string, seq int, skipped bool) map[string]any {
	return map[string]any{"pane_id": pane, "terminal_id": "term_" + pane, "workspace_id": "w1", "tab_id": "w1:t1", "focused": false,
		"agent": agent, "agent_status": status, "state_change_seq": seq, "screen_detection_skipped": skipped, "revision": 1}
}

func procInfo(pane string, pid int, name string) map[string]any {
	return map[string]any{"type": "pane_process_info", "process_info": map[string]any{"pane_id": pane, "shell_pid": pid - 1,
		"foreground_processes": []map[string]any{{"pid": 1, "name": "zsh"}, {"pid": pid, "name": name, "cmdline": name + " --resume"}}}}
}

func newPopulated(t *testing.T) (*fakeHerdr, *Runtime) {
	f := newFake(t)
	f.set("agent.list", agentList(
		info("w1:p1", "claude", "working", 4, true),
		info("w1:p2", "claude", "blocked", 7, false),
		info("w1:p3", "claude", "idle", 2, false),
		info("w1:p4", "claude", "unknown", 1, false),
		info("w1:p5", "codex", "working", 1, false)))
	return f, New(Config{Socket: f.path, Timeout: time.Second, BackoffMin: 10 * time.Millisecond, BackoffMax: 40 * time.Millisecond})
}

func TestSessionsBindPaneToForegroundPid(t *testing.T) {
	f := newFake(t)
	f.set("agent.list", agentList(info("w1:p1", "claude", "working", 4, false)))
	f.set("pane.process_info", procInfo("w1:p1", 4242, "claude"))
	got := New(Config{Socket: f.path}).Sessions(t0, claimClaude)
	if len(got) != 1 || got[0].PID != 4242 || got[0].PPID != 4241 || got[0].Runtime != "herdr" || got[0].Agent != "claudecode" ||
		got[0].Address != "w1:p1" || got[0].Cmdline != "claude --resume" || !got[0].Start.IsZero() {
		t.Fatalf("session %+v", got)
	}
}

func TestStatesAndRanks(t *testing.T) {
	cases := []struct {
		status  string
		skipped bool
		explain map[string]any
		state   string
		rank    int
		has     bool
	}{
		{"working", false, nil, "working", RankScreen, true},
		{"working", true, nil, "working", host.RankNative, true},
		{"idle", false, nil, "waiting", RankScreen, true},
		{"done", true, nil, "waiting", host.RankNative, true},
		{"blocked", false, map[string]any{"matched_rule": map[string]any{"id": "claude_permission_prompt"}}, "approval", RankScreen, true},
		{"blocked", false, map[string]any{"matched_rule": "question_choice"}, "question", RankScreen, true},
		{"blocked", false, map[string]any{"unrelated": 1}, "approval", RankScreen, true},
		{"unknown", false, nil, "", 0, false},
	}
	for _, c := range cases {
		f := newFake(t)
		f.set("agent.list", agentList(info("w1:p1", "claude", c.status, 3, c.skipped)))
		f.set("pane.process_info", procInfo("w1:p1", 99, "claude"))
		if c.explain != nil {
			f.set("agent.explain", map[string]any{"type": "agent_explain", "explain": c.explain})
		}
		r := New(Config{Socket: f.path})
		if s := r.Sessions(t0, claimClaude); len(s) != 1 {
			t.Fatalf("%v: sessions %v", c.status, s)
		}
		obs := r.Observe(t0, []host.Bound{{Session: host.Session{PID: 99}}, {Session: host.Session{PID: 5}}})
		if len(obs) != 2 || len(obs[1].States) != 0 {
			t.Fatalf("%v: obs %+v", c.status, obs)
		}
		if !c.has {
			if len(obs[0].States) != 0 {
				t.Errorf("%v: unexpected %+v", c.status, obs[0].States)
			}
			continue
		}
		s := obs[0].States[0]
		if s.State != c.state || s.Rank != c.rank || s.Source != "herdr" {
			t.Errorf("%v skipped=%v: got %+v", c.status, c.skipped, s)
		}
	}
}

func TestStateSinceMovesOnlyWhenTheStateChanges(t *testing.T) {
	f := newFake(t)
	f.set("agent.list", agentList(info("w1:p1", "claude", "working", 3, false)))
	f.set("pane.process_info", procInfo("w1:p1", 99, "claude"))
	r := New(Config{Socket: f.path})
	at := func(now time.Time) time.Time {
		r.Sessions(now, claimClaude)
		return r.Observe(now, []host.Bound{{Session: host.Session{PID: 99}}})[0].States[0].ObservedAt
	}
	if got := at(t0); !got.IsZero() {
		t.Fatalf("first sight is not a change: %v", got)
	}
	if got := at(t0.Add(time.Minute)); !got.IsZero() {
		t.Fatalf("unchanged: %v", got)
	}
	f.set("agent.list", agentList(info("w1:p1", "claude", "idle", 4, false)))
	if got := at(t0.Add(2 * time.Minute)); !got.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("changed: %v", got)
	}
	if got := at(t0.Add(3 * time.Minute)); !got.Equal(t0.Add(2 * time.Minute)) {
		t.Fatalf("settled: %v", got)
	}
}

func TestPaneWithoutAClaimedProcessIsSkipped(t *testing.T) {
	f := newFake(t)
	f.set("agent.list", agentList(info("w1:p1", "codex", "working", 1, false)))
	f.set("pane.process_info", procInfo("w1:p1", 99, "codex"))
	if s := New(Config{Socket: f.path}).Sessions(t0, claimClaude); len(s) != 0 {
		t.Fatalf("%v", s)
	}
}

func TestMissingSocketIsQuietlyNotRunning(t *testing.T) {
	var logged atomic.Int32
	r := New(Config{Socket: "/nonexistent/herdr.sock", BackoffMin: time.Millisecond, BackoffMax: 2 * time.Millisecond,
		Logf: func(string, ...any) { logged.Add(1) }})
	if s := r.Sessions(t0, claimClaude); s != nil {
		t.Fatalf("%v", s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	if err := r.Watch(ctx, func() { t.Error("poked with no server") }); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("watch returned %v", err)
	}
	if logged.Load() != 0 {
		t.Fatalf("logged %d lines for a server that is simply not running", logged.Load())
	}
}

func TestClientOnlyIssuesAllowlistedMethods(t *testing.T) {
	f, r := newPopulated(t)
	f.set("pane.process_info", procInfo("w1:p1", 4242, "claude"))
	f.set("agent.explain", map[string]any{"type": "agent_explain", "explain": map[string]any{}})
	r.Sessions(t0, claimClaude)

	ctx, cancel := context.WithCancel(context.Background())
	pokes := make(chan struct{}, 8)
	go r.Watch(ctx, func() { pokes <- struct{}{} })
	select {
	case <-f.subs:
	case <-time.After(2 * time.Second):
		t.Fatal("no subscription")
	}
	cancel()

	seen := f.methods()
	if len(seen) < 4 {
		t.Fatalf("exercised too little: %v", seen)
	}
	for _, m := range seen {
		if !allowedMethods[m] {
			t.Errorf("client sent %q", m)
		}
	}
	c, err := dial(f.path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.close()
	for _, m := range []string{"agent.prompt", "agent.send_keys", "pane.send_text", "pane.send_keys", "pane.send_input", "agent.start", "pane.close", "server.stop", "pane.report_agent"} {
		if _, err := c.send(m, struct{}{}); !errors.Is(err, errDisallowed) {
			t.Errorf("%s was not refused: %v", m, err)
		}
	}
	for m := range allowedMethods {
		for _, bad := range []string{"send", "prompt", "start", "close", "stop", "report", "write", "set"} {
			if strings.Contains(m, bad) {
				t.Errorf("allowlisted method %q looks like a write", m)
			}
		}
	}
}

func TestSourceNamesNoWriteMethod(t *testing.T) {
	for _, file := range []string{"client.go", "herdr.go", "state.go", "watch.go"} {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for n, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, bad := range []string{`"pane.send`, `"agent.prompt`, `"agent.send`, `"agent.start`, `"pane.report`, `"pane.close"`} {
				if strings.Contains(line, bad) {
					t.Errorf("%s:%d names %s", file, n+1, bad)
				}
			}
		}
	}
}

func TestWatchPokesOnStatusChangeAndResubscribesWhenPanesChange(t *testing.T) {
	f := newFake(t)
	f.set("agent.list", agentList(info("w1:p1", "claude", "working", 1, false)))
	r := New(Config{Socket: f.path, Timeout: time.Second, BackoffMin: 5 * time.Millisecond, BackoffMax: 20 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var pokes atomic.Int32
	done := make(chan error, 1)
	go func() { done <- r.Watch(ctx, func() { pokes.Add(1) }) }()

	subs := <-f.subs
	types := map[string]string{}
	for _, s := range subs {
		types[s["type"].(string)+"/"+str(s["pane_id"])] = "x"
	}
	for _, want := range []string{"pane.created/", "pane.closed/", "pane.exited/", "pane.agent_detected/", "pane.agent_status_changed/w1:p1"} {
		if types[want] == "" {
			t.Errorf("missing subscription %s in %v", want, types)
		}
	}
	waitFor(t, func() bool { return pokes.Load() == 1 }, "poke after subscribing")

	f.send(`{"event":"pane.agent_status_changed","data":{"pane_id":"w1:p1","workspace_id":"w1","agent_status":"blocked"}}`)
	waitFor(t, func() bool { return pokes.Load() == 2 }, "poke on status change")

	f.set("agent.list", agentList(info("w1:p1", "claude", "working", 1, false), info("w1:p2", "claude", "idle", 1, false)))
	f.send(`{"event":"pane_created","data":{"type":"pane_created","pane":{"pane_id":"w1:p2"}}}`)
	subs = <-f.subs
	found := false
	for _, s := range subs {
		found = found || str(s["pane_id"]) == "w1:p2"
	}
	if !found {
		t.Errorf("new pane not subscribed: %v", subs)
	}

	waitFor(t, func() bool { return pokes.Load() == 4 }, "poke after resubscribing")
	// events_lost closes the stream; the watcher reconnects and polls again.
	before := pokes.Load()
	f.send(`{"id":"fleet_2","error":{"code":"events_lost","message":"behind"}}`)
	<-f.subs
	waitFor(t, func() bool { return pokes.Load() > before }, "poke after reconnect")

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("watch returned %v", err)
	}
}

func str(v any) string { s, _ := v.(string); return s }

func waitFor(t *testing.T, ok func() bool, what string) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", what)
}

func TestFramesTolerateUnknownFieldsAndErrors(t *testing.T) {
	f := newFake(t)
	f.errFor["agent.list"] = map[string]string{"code": "not_found", "message": "nope"}
	r := New(Config{Socket: f.path})
	if s := r.Sessions(t0, claimClaude); s != nil {
		t.Fatalf("%v", s)
	}
}

func TestClassifyExplain(t *testing.T) {
	for in, want := range map[string]string{
		`{"matched_rule":"claude.question.choice"}`:       "question",
		`{"matched_rule":{"name":"permission_prompt"}}`:   "approval",
		`{"final_state":"blocked","evidence":"question"}`: "approval",
		`not json`: "approval",
	} {
		if got := classifyExplain([]byte(in)); got != want {
			t.Errorf("%s: %s, want %s", in, got, want)
		}
	}
}

// loadFrame returns the result of a response frame captured from herdr v0.9.5
// on host-a (testdata/live-0.9.5-*.json).
func loadFrame(t *testing.T, method string) json.RawMessage {
	t.Helper()
	b, err := os.ReadFile("testdata/live-0.9.5-" + strings.ReplaceAll(method, ".", "_") + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(b, &f); err != nil || len(f.Result) == 0 {
		t.Fatalf("%s: %v", method, err)
	}
	return f.Result
}

// TestLiveCapturedFramesBindAndClassify replays a real blocked Claude pane. The
// fake closes each connection after one answer, as herdr does; before the fix
// the second request of a pass hit a broken pipe and no session appeared.
func TestLiveCapturedFramesBindAndClassify(t *testing.T) {
	f := newFake(t)
	for _, m := range []string{"agent.list", "pane.process_info", "agent.explain"} {
		f.set(m, loadFrame(t, m))
	}
	r := New(Config{Socket: f.path, Timeout: time.Second})
	got := r.Sessions(t0, claimClaude)
	if len(got) != 1 || got[0].PID != 2174268 || got[0].Address != "w1:p1" || got[0].Runtime != "herdr" {
		t.Fatalf("session %+v", got)
	}
	obs := r.Observe(t0, []host.Bound{{Session: host.Session{PID: 2174268}}})
	if len(obs) != 1 || len(obs[0].States) != 1 || obs[0].States[0].State != "approval" || obs[0].States[0].Source != "herdr" {
		t.Fatalf("observation %+v", obs)
	}
}
