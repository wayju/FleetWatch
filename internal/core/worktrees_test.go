package core

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/agent/claudecode"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/forge"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixture builds <root>/<repo>/.git with the given worktree admin dirs and
// returns the checkout path each one points at.
type wtFix struct {
	id, branch string // branch "" writes a detached HEAD
	rel        string // checkout path relative to the repo
	gitAge     time.Duration
}

var wtNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeRepo(t *testing.T, root, repo, remote string, wts []wtFix, now time.Time) {
	t.Helper()
	gd := filepath.Join(root, repo, ".git")
	mustWrite(t, filepath.Join(gd, "config"), "[remote \"origin\"]\n\turl = "+remote+"\n")
	for _, w := range wts {
		admin := filepath.Join(gd, "worktrees", w.id)
		mustWrite(t, filepath.Join(admin, "gitdir"), filepath.Join(root, repo, w.rel, ".git")+"\n")
		head := "ref: refs/heads/" + w.branch + "\n"
		if w.branch == "" {
			head = strings.Repeat("a", 40) + "\n"
		}
		mustWrite(t, filepath.Join(admin, "HEAD"), head)
		mustWrite(t, filepath.Join(admin, "index"), "x")
		old := now.Add(-w.gitAge)
		for _, f := range []string{"HEAD", "index"} {
			if err := os.Chtimes(filepath.Join(admin, f), old, old); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDiscoverWorktrees(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "stack", "https://git.example.test/acme/example-stack.git", []wtFix{
		{"fix-3167", "fix/3167-nested-name", ".claude/worktrees/fix/3167-nested-name", time.Hour},
		{"detached", "", ".claude/worktrees/det", time.Hour},
	}, wtNow)
	// An admin dir whose gitdir points outside the src root is not believed.
	mustWrite(t, filepath.Join(root, "stack", ".git", "worktrees", "evil", "gitdir"), "/etc/.git\n")
	mustWrite(t, filepath.Join(root, "stack", ".git", "worktrees", "evil", "HEAD"), "ref: refs/heads/x\n")
	// A symlinked repo directory is not followed.
	outside := t.TempDir()
	makeRepo(t, outside, "r", "https://git.example.test/a/b.git", []wtFix{{"w", "b", "w", 0}}, wtNow)
	if err := os.Symlink(filepath.Join(outside, "r"), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	// A plain checkout with no worktrees contributes nothing.
	mustWrite(t, filepath.Join(root, "plain", ".git", "config"), "")

	got := discoverWorktrees([]config.WorktreeRoot{{Path: root, Depth: 1}})
	if len(got) != 2 {
		t.Fatalf("want 2 worktrees, got %d: %+v", len(got), got)
	}
	by := map[string]foundWorktree{}
	for _, g := range got {
		by[g.name] = g
	}
	f := by["fix-3167"]
	if f.branch != "fix/3167-nested-name" || f.repoSlug != "acme/example-stack" || f.repoHost != "git.example.test" {
		t.Fatalf("bad fix worktree: %+v", f)
	}
	if f.path != filepath.Join(root, "stack", ".claude/worktrees/fix/3167-nested-name") {
		t.Fatalf("path %q", f.path)
	}
	if by["detached"].branch != "" {
		t.Fatalf("detached HEAD must have no branch: %+v", by["detached"])
	}
	if f.gitAt.IsZero() || wtNow.Sub(f.gitAt) < 59*time.Minute {
		t.Fatalf("git activity not read: %v", f.gitAt)
	}
}

func TestInvalidRefRejected(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/a/b.git", []wtFix{{"w", "x/../../etc", "w", 0}}, wtNow)
	if got := discoverWorktrees([]config.WorktreeRoot{{Path: root, Depth: 1}}); len(got) != 1 || got[0].branch != "" {
		t.Fatalf("traversal ref must be dropped: %+v", got)
	}
}

func tr(id, cwd string, last time.Time, tools ...*ToolCall) *Transcript {
	t := newTranscript(id)
	t.Cwd, t.LastActivity, t.Tools = cwd, last, tools
	return t
}

func edit(path string, at time.Time) *ToolCall {
	return &ToolCall{Name: "Edit", Target: path, At: at}
}

// touchSession is a session that only knows where it worked.
type touchSession struct {
	agent.Session
	touches []agent.Touch
}

func (s touchSession) Touches() []agent.Touch { return s.touches }

// touchSrc is the session whose transcripts are t (main thread) and subs.
func touchSrc(t *Transcript, subs map[string]*Transcript) agent.Session {
	out := claudecode.TranscriptTouches(t)
	for id, sub := range subs {
		for _, x := range claudecode.TranscriptTouches(sub) {
			x.Agent = id
			out = append(out, x)
		}
	}
	return touchSession{touches: out}
}

func newIndex(root string, g *forge.Lookup) *worktreeIndex {
	return newWorktreeIndex([]config.WorktreeRoot{{Path: root, Depth: 1}}, g, newIssueScan(""))
}

func TestWriterAttributionLongestPrefix(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "stack", "https://git.example.test/acme/stack.git", []wtFix{
		{"a", "feat/1-a", ".claude/worktrees/feat/1-a", time.Hour},
		{"ab", "feat/1-a-b", ".claude/worktrees/feat/1-a-b", time.Hour},
		{"inner", "feat/9-inner", ".claude/worktrees/feat/1-a/inner", time.Hour},
	}, wtNow)
	base := filepath.Join(root, "stack", ".claude/worktrees/feat")
	sess := SessionView{ID: "sess-1", Slot: "3", Agents: []AgentView{{ID: "ag1", Description: "fix the thing"}}}
	main := tr("sess-1", filepath.Join(base, "1-a"), wtNow.Add(-5*time.Minute))
	sub := tr("ag1", filepath.Join(root, "stack"), wtNow.Add(-time.Minute),
		edit(filepath.Join(base, "1-a-b/src/x.go"), wtNow.Add(-2*time.Minute)),
		edit(filepath.Join(base, "1-a/inner/y.go"), wtNow.Add(-3*time.Minute)),
		&ToolCall{Name: "Bash", Target: filepath.Join(base, "1-a/z"), At: wtNow}, // not a file tool
	)
	src := map[string]agent.Session{"sess-1": touchSrc(main, map[string]*Transcript{"ag1": sub})}
	views := newIndex(root, nil).view(wtNow, []SessionView{sess}, src)
	by := map[string]WorktreeView{}
	for _, v := range views {
		by[v.Name] = v
	}
	if w := by["a"].Writers; len(w) != 1 || w[0].AgentID != "" || w[0].Session != "sess-1" || w[0].Slot != "3" || w[0].LastAt != wtNow.Add(-5*time.Minute).Unix() {
		t.Fatalf("a writers: %+v", w)
	}
	if w := by["ab"].Writers; len(w) != 1 || w[0].AgentID != "ag1" || w[0].AgentDesc != "fix the thing" || w[0].LastAt != wtNow.Add(-2*time.Minute).Unix() {
		t.Fatalf("ab (sibling prefix must not leak into a): %+v", w)
	}
	if w := by["inner"].Writers; len(w) != 1 || w[0].AgentID != "ag1" {
		t.Fatalf("inner nested worktree should win its files: %+v", w)
	}
	if by["a"].Issue == nil || *by["a"].Issue != 1 || by["inner"].Branch != "feat/9-inner" {
		t.Fatalf("issue/branch: %+v %+v", by["a"], by["inner"])
	}
	if by["a"].PathRel != ".claude/worktrees/feat/1-a" {
		t.Fatalf("path_rel %q", by["a"].PathRel)
	}
}

func TestSlashBranchIssue(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{
		{"w", "fix/3167-deep/name", "w", 0}, {"n", "chore/no-number", "n", 0},
	}, wtNow)
	for _, v := range newIndex(root, nil).view(wtNow, nil, nil) {
		switch v.Name {
		case "w":
			if v.Issue == nil || *v.Issue != 3167 {
				t.Fatalf("issue from nested branch: %+v", v.Issue)
			}
		case "n":
			if v.Issue != nil {
				t.Fatalf("no issue expected: %d", *v.Issue)
			}
		}
	}
}

func TestActiveStaleTwoWriters(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{
		{"busy", "feat/1-busy", "busy", 48 * time.Hour},
		{"quiet", "feat/2-quiet", "quiet", 2 * time.Hour},
		{"old", "feat/3-old", "old", 30 * time.Hour},
		{"wrote", "feat/4-wrote", "wrote", 30 * time.Hour},
	}, wtNow)
	p := func(n string) string { return filepath.Join(root, "r", n) }
	sessions := []SessionView{{ID: "s1"}, {ID: "s2"}}
	srcs := map[string]agent.Session{
		"s1": touchSrc(tr("s1", p("busy"), wtNow.Add(-2*time.Minute)), nil),
		"s2": touchSrc(tr("s2", p("busy"), wtNow.Add(-9*time.Minute), edit(p("wrote/f.go"), wtNow.Add(-3*time.Hour))), nil),
	}
	by := map[string]WorktreeView{}
	for _, v := range newIndex(root, nil).view(wtNow, sessions, srcs) {
		by[v.Name] = v
	}
	if b := by["busy"]; !b.Active || !b.TwoWriters || b.Stale {
		t.Fatalf("busy: %+v", b)
	}
	if q := by["quiet"]; q.Active || q.Stale || q.TwoWriters || len(q.Writers) != 0 {
		t.Fatalf("quiet (git 2h ago, no writer): %+v", q)
	}
	if o := by["old"]; !o.Stale || o.Active {
		t.Fatalf("old must be stale: %+v", o)
	}
	if w := by["wrote"]; w.Stale || w.Active || len(w.Writers) != 1 {
		t.Fatalf("a writer 3h ago keeps it fresh: %+v", w)
	}
	// Two writers means within ten minutes: an eleven-minute-old second writer does not count.
	srcs["s2"] = touchSrc(tr("s2", p("busy"), wtNow.Add(-11*time.Minute)), nil)
	if b := newIndex(root, nil).view(wtNow, sessions, srcs); !b[0].Active || b[0].TwoWriters {
		t.Fatalf("one recent writer only: %+v", b[0])
	}
}

func TestMergedUnknownWithoutToken(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{{"w", "feat/1-x", "w", 0}}, wtNow)
	for name, g := range map[string]*forge.Lookup{
		"nil client": nil,
		"no token":   newGiteaClient("http://127.0.0.1:1", "", ""),
	} {
		v := newIndex(root, g).view(wtNow, nil, nil)[0]
		if v.Merged != nil {
			t.Fatalf("%s: merged must be null, got %v", name, *v.Merged)
		}
		b, _ := json.Marshal(v)
		if !strings.Contains(string(b), `"merged":null`) {
			t.Fatalf("%s: %s", name, b)
		}
	}
}

func TestMergedFromForgeEndToEnd(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token tok" || !strings.HasPrefix(r.URL.Path, "/api/v1/repos/acme/r/pulls") {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		w.Write([]byte(`[{"merged":true,"head":{"ref":"feat/1-x","repo":{"full_name":"acme/r"}}}]`))
	}))
	defer srv.Close()
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{{"w", "feat/1-x", "w", 0}, {"v", "feat/2-y", "v", 0}}, wtNow)
	idx := newIndex(root, newGiteaClient(srv.URL, "tok", ""))
	deadline := time.Now().Add(5 * time.Second)
	for {
		by := map[string]*bool{}
		for _, v := range idx.view(wtNow, nil, nil) {
			by[v.Branch] = v.Merged
		}
		if by["feat/1-x"] != nil && by["feat/2-y"] != nil {
			if !*by["feat/1-x"] || *by["feat/2-y"] {
				t.Fatalf("merged flags: %v %v", *by["feat/1-x"], *by["feat/2-y"])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("forge answer never landed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDiscoverDepthAndMultipleRoots(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	makeRepo(t, filepath.Join(a, "group"), "deep", "https://github.com/o/deep.git", []wtFix{{"w", "1-x", "w", 0}}, wtNow)
	makeRepo(t, b, "flat", "https://github.com/o/flat.git", []wtFix{{"w", "2-y", "w", 0}}, wtNow)
	if got := discoverWorktrees([]config.WorktreeRoot{{Path: a, Depth: 1}}); len(got) != 0 {
		t.Fatalf("depth 1 must not reach a grouped repo: %+v", got)
	}
	got := discoverWorktrees([]config.WorktreeRoot{{Path: a, Depth: 2}, {Path: b, Depth: 1}})
	if len(got) != 2 {
		t.Fatalf("want 2 worktrees across both roots, got %+v", got)
	}
}

func TestRescanCadence(t *testing.T) {
	root := t.TempDir()
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{{"one", "feat/1-a", "one", 0}}, wtNow)
	idx := newIndex(root, nil)
	if n := len(idx.list(wtNow)); n != 1 {
		t.Fatalf("got %d", n)
	}
	makeRepo(t, root, "r", "https://git.example.test/acme/r.git", []wtFix{{"two", "feat/2-b", "two", 0}}, wtNow)
	if n := len(idx.list(wtNow.Add(30 * time.Second))); n != 1 {
		t.Fatalf("rescanned inside 60s: %d", n)
	}
	if n := len(idx.list(wtNow.Add(61 * time.Second))); n != 2 {
		t.Fatalf("no rescan after 60s: %d", n)
	}
}
