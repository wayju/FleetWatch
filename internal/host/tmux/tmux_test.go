package tmux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// fakeProc writes a /proc/<pid>/stat and cmdline for a fake process.
func fakeProc(t *testing.T, root string, pid, ppid int, comm string) {
	t.Helper()
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[1] = fmt.Sprint(ppid)
	dir := filepath.Join(root, fmt.Sprint(pid))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "stat"), []byte(fmt.Sprintf("%d (%s) %s", pid, comm, strings.Join(fields, " "))), 0o644)
	os.WriteFile(filepath.Join(dir, "cmdline"), []byte(comm+"\x00"), 0o644)
}

func claimClaude(comm string, _ func() string) (string, bool) { return "claudecode", comm == "claude" }

// recorded answers tmux calls from fixtures and records what was asked.
type recorded struct {
	calls  [][]string
	screen map[string]string // pane id -> capture-pane output
}

func (r *recorded) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, args)
	for _, a := range args {
		switch a {
		case "list-panes":
			b, _ := os.ReadFile(filepath.Join("testdata", "list-panes.txt"))
			return b, nil
		case "capture-pane":
			return []byte(r.screen[args[len(args)-1]]), nil
		}
	}
	return nil, fmt.Errorf("unexpected call %v", args)
}

// tree: pane 4001 runs a shell (4001) -> node wrapper (4010) -> claude (4011);
// pane 4002 is the agent directly; pane 4003 runs only a shell; claude 4999
// runs outside tmux.
func tree(t *testing.T) string {
	root := t.TempDir()
	fakeProc(t, root, 4001, 1, "bash")
	fakeProc(t, root, 4010, 4001, "sh")
	fakeProc(t, root, 4011, 4010, "claude")
	fakeProc(t, root, 4002, 1, "claude")
	fakeProc(t, root, 4003, 1, "zsh")
	fakeProc(t, root, 4999, 1, "claude")
	return root
}

func TestSessionsMapPanePidToAgent(t *testing.T) {
	rec := &recorded{}
	rt := New(Config{ProcRoot: tree(t), Run: rec.run})
	got := rt.Sessions(time.Now(), claimClaude)
	if len(got) != 2 {
		t.Fatalf("%+v", got)
	}
	by := map[int]host.Session{}
	for _, s := range got {
		by[s.PID] = s
	}
	if s := by[4011]; s.Address != "%18" || s.Location != "main:2.0" || s.Runtime != "tmux" || s.Agent != "claudecode" {
		t.Errorf("walked agent: %+v", s)
	}
	if s := by[4002]; s.Address != "%17" || s.Location != "main:4.0" {
		t.Errorf("direct agent: %+v", s)
	}
	if _, ok := by[4999]; ok {
		t.Error("an agent outside tmux must not be reported")
	}
	if first := rec.calls[0]; first[0] != "list-panes" || first[1] != "-a" || first[2] != "-F" {
		t.Errorf("list call %v", first)
	}
}

func TestSocketsAreAskedWithDashS(t *testing.T) {
	rec := &recorded{}
	rt := New(Config{ProcRoot: tree(t), Sockets: []string{"/run/a", "/run/b"}, Run: rec.run})
	rt.Sessions(time.Now(), claimClaude)
	if len(rec.calls) != 2 || rec.calls[0][0] != "-S" || rec.calls[0][1] != "/run/a" || rec.calls[1][1] != "/run/b" {
		t.Fatalf("%v", rec.calls)
	}
}

func TestUnreachableServerReportsNothing(t *testing.T) {
	fail := func(context.Context, string, ...string) ([]byte, error) { return nil, fmt.Errorf("no server running") }
	rt := New(Config{ProcRoot: tree(t), Run: fail})
	if got := rt.Sessions(time.Now(), claimClaude); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestParsePanesSkipsMalformedLines(t *testing.T) {
	ps := parsePanes("", fixture(t, "list-panes.txt"))
	if len(ps) != 3 || ps[2].id != "%40" || ps[2].pid != 4003 || ps[2].label != "main:9.0" {
		t.Fatalf("%+v", ps)
	}
}

func rulesFor(t *testing.T) Rules {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "tmux-rules.claude-code.json"))
	if err != nil {
		t.Fatal(err)
	}
	rs, err := ParseRules(b)
	if err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestObserveScrapesRecordedScreens(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	cases := []struct{ file, state string }{
		{"claudecode-approval.txt", "approval"},
		{"claudecode-working.txt", "working"},
		{"claudecode-idle.txt", "waiting"},
		{"claudecode-idle-agents.txt", "waiting"},
		{"shell.txt", ""},
	}
	for _, c := range cases {
		rec := &recorded{screen: map[string]string{"%18": fixture(t, c.file)}}
		rt := New(Config{ProcRoot: tree(t), Run: rec.run, Scrape: true, ScrapeLines: 30, Rules: rulesFor(t), Poll: 2 * time.Second})
		rt.Sessions(now, claimClaude)
		obs := rt.Observe(now, []host.Bound{{Session: host.Session{PID: 4011, Agent: "claudecode"}}})
		if obs[0].Address != "%18" {
			t.Errorf("%s: address %q", c.file, obs[0].Address)
		}
		if c.state == "" {
			if len(obs[0].States) != 0 {
				t.Errorf("%s: %+v", c.file, obs[0].States)
			}
			continue
		}
		if len(obs[0].States) != 1 {
			t.Fatalf("%s: %+v", c.file, obs[0].States)
		}
		s := obs[0].States[0]
		if s.State != c.state || s.Rank != host.RankPane || s.Source != "pane" || !s.FreshUntil.Equal(now.Add(4*time.Second)) {
			t.Errorf("%s: %+v", c.file, s)
		}
	}
}

func TestScrapeAsksForTheLastLinesOfThePane(t *testing.T) {
	rec := &recorded{screen: map[string]string{}}
	rt := New(Config{ProcRoot: tree(t), Run: rec.run, Scrape: true, ScrapeLines: 15, Rules: rulesFor(t)})
	rt.Sessions(time.Now(), claimClaude)
	rt.Observe(time.Now(), []host.Bound{{Session: host.Session{PID: 4011, Agent: "claudecode"}}})
	last := strings.Join(rec.calls[len(rec.calls)-1], " ")
	if last != "capture-pane -p -J -S -15 -t %18" {
		t.Fatalf("%q", last)
	}
}

func TestScrapeOffNeverCaptures(t *testing.T) {
	rec := &recorded{}
	rt := New(Config{ProcRoot: tree(t), Run: rec.run, Rules: rulesFor(t)})
	rt.Sessions(time.Now(), claimClaude)
	obs := rt.Observe(time.Now(), []host.Bound{{Session: host.Session{PID: 4011, Agent: "claudecode"}}})
	for _, c := range rec.calls {
		if c[0] == "capture-pane" {
			t.Fatal("captured with scrape off")
		}
	}
	if obs[0].Address != "%18" || len(obs[0].States) != 0 {
		t.Fatalf("%+v", obs[0])
	}
}

func TestApprovalInScrollbackIsNotApproval(t *testing.T) {
	// An approval prompt that scrolled up (the agent has moved on) must not match.
	screen := fixture(t, "claudecode-approval.txt") + fixture(t, "claudecode-working.txt")
	if r, ok := rulesFor(t).match("claudecode", trimScreen(screen)); !ok || r.State != "working" {
		t.Fatalf("%+v %v", r, ok)
	}
}

func TestParseRulesIsStrict(t *testing.T) {
	for name, in := range map[string]string{
		"unknown key":   `{"a":[{"state":"working","regex":"x","oops":1}]}`,
		"unknown state": `{"a":[{"state":"sleeping","regex":"x"}]}`,
		"bad regex":     `{"a":[{"state":"working","regex":"("}]}`,
		"empty regex":   `{"a":[{"state":"working","regex":""}]}`,
		"trailing":      `{} {}`,
	} {
		if _, err := ParseRules([]byte(in)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
