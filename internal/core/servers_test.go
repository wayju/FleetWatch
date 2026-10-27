package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// srvJSON is one scanner row; claude 100 owns them unless claude says otherwise.
func srvRow(claude, pid, port int, startedAgo time.Duration) string {
	return fmt.Sprintf(`{"claude_pid":%d,"pid":%d,"port":%d,"cmd":"python3 -m http.server %d","cwd":"/home/u/src/app","dir":"/work/tree","started_at":%d}`,
		claude, pid, port, port, t0.Add(-startedAgo).Unix())
}

func srvDoc(scannedAt time.Time, rows ...string) string {
	return fmt.Sprintf(`{"scanned_at":%d,"servers":[%s]}`, scannedAt.Unix(), strings.Join(rows, ","))
}

func TestMarkServersStale(t *testing.T) {
	yes := true
	src := t.TempDir()
	app := filepath.Join(src, "app")
	live := filepath.Join(app, ".claude", "worktrees", "feature", "1-x")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	wts := []WorktreeView{{RepoPath: app, PathRel: ".claude/worktrees/feature/1-x", Merged: &yes}}
	recent := t0.Add(-time.Hour).Unix()
	servers := []ServerView{
		{PID: 1, Dir: "/fresh", StartedAt: recent},
		{PID: 2, Dir: "/old", StartedAt: t0.Add(-13 * time.Hour).Unix()},
		{PID: 3, Dir: filepath.Join(live, "sub"), StartedAt: recent},
		{PID: 4, Dir: filepath.Join(app, ".claude", "worktrees", "gone"), StartedAt: recent},
		{PID: 5, Dir: app, StartedAt: recent},
	}
	markServersStale(servers, t0, wts, []string{src})
	want := []string{"", "age", "merged", "dir_gone", ""}
	for i, s := range servers {
		if s.StaleReason != want[i] || s.Stale != (want[i] != "") {
			t.Errorf("server %d: stale=%v reason=%q, want %q", s.PID, s.Stale, s.StaleReason, want[i])
		}
	}
}
