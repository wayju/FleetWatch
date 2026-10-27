package proc

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var boot = time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)

func fake(t *testing.T, root string, pid, ppid int, comm, cmdline string, ticks int64) {
	t.Helper()
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[1], fields[19] = fmt.Sprint(ppid), fmt.Sprint(ticks)
	dir := filepath.Join(root, fmt.Sprint(pid))
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "stat"), []byte(fmt.Sprintf("%d (%s) %s", pid, comm, strings.Join(fields, " "))), 0o644)
	os.WriteFile(filepath.Join(dir, "cmdline"), []byte(strings.ReplaceAll(cmdline, " ", "\x00")+"\x00"), 0o644)
}

func root(t *testing.T) string {
	r := t.TempDir()
	os.WriteFile(filepath.Join(r, "stat"), []byte(fmt.Sprintf("cpu 0\nbtime %d\n", boot.Unix())), 0o644)
	return r
}

func TestScanFoldsNestedAndOrdersByPID(t *testing.T) {
	r := root(t)
	fake(t, r, 500, 1, "node", "node x/@anthropic-ai/claude-code/cli.js", 100)
	fake(t, r, 501, 500, "claude", "claude", 100)
	fake(t, r, 200, 1, "claude", "claude", 360000)
	os.MkdirAll(filepath.Join(r, "self"), 0o755) // non-numeric entries are skipped
	claim := func(comm string, cmdline func() string) (string, bool) {
		return "a", comm == "claude" || strings.Contains(cmdline(), "claude-code")
	}
	got := Scan(r, claim)
	if len(got) != 2 || got[0].PID != 200 || got[1].PID != 500 {
		t.Fatalf("%+v", got)
	}
	if !got[0].Start.Equal(boot.Add(3600*time.Second)) || got[0].Runtime != "proc" || got[0].Agent != "a" {
		t.Fatalf("session %+v", got[0])
	}
}

func TestScanReadsCmdlineOnlyWhenAsked(t *testing.T) {
	r := root(t)
	fake(t, r, 10, 1, "bash", "bash", 1)
	fake(t, r, 11, 1, "claude", "claude", 1)
	os.Remove(filepath.Join(r, "10", "cmdline")) // a read would fail, but must not be attempted
	asked := 0
	Scan(r, func(comm string, cmdline func() string) (string, bool) {
		if comm == "claude" {
			return "a", true
		}
		asked++
		return "", false
	})
	if asked != 1 {
		t.Fatalf("claimer called %d times", asked)
	}
}

func TestFileAllowlist(t *testing.T) {
	r := root(t)
	fake(t, r, 1, 0, "x", "x", 1)
	for _, name := range []string{"stat", "cmdline"} {
		if _, err := File(r, 1, name); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, name := range []string{"environ", "cwd", "fd", "exe", "maps", "../1/stat"} {
		if _, err := File(r, 1, name); err == nil {
			t.Errorf("%s must be refused", name)
		}
	}
}

func TestScanMissingRoot(t *testing.T) {
	if Scan(filepath.Join(t.TempDir(), "nope"), nil) != nil {
		t.Error("missing root")
	}
}
