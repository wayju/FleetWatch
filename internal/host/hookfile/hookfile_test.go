package hookfile

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

var t0 = time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)

func put(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func file(sid string, pid int, state string, at time.Time) string {
	return fmt.Sprintf(`{"schema":1,"session_id":%q,"pid":%d,"state":%q,"reason":"r","hook_event":"Stop","pane":"%%5","observed_at":%q}`,
		sid, pid, state, at.UTC().Format(time.RFC3339))
}

func bound(pid int, sid string, start time.Time) host.Bound {
	return host.Bound{Session: host.Session{PID: pid, Start: start}, SessionID: sid}
}

func TestObserveGivesANativeSignalWithFreshUntilAndThePane(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "S1.json", file("S1", 10, "approval", t0))
	rt := New(Config{Dir: dir, Fresh: 10 * time.Minute})
	o := rt.Observe(t0.Add(time.Second), []host.Bound{bound(10, "S1", t0.Add(-time.Hour))})[0]
	if o.Address != "%5" || len(o.States) != 1 {
		t.Fatalf("%+v", o)
	}
	s := o.States[0]
	if s.State != "approval" || s.Rank != host.RankNative || s.Source != "hook" || !s.ObservedAt.Equal(t0) || !s.FreshUntil.Equal(t0.Add(10*time.Minute)) {
		t.Fatalf("%+v", s)
	}
}

func TestSessionIDBeatsPIDAndPIDFindsAnUnboundSession(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "A.json", file("A", 99, "working", t0))
	put(t, dir, "B.json", file("B", 11, "waiting", t0))
	rt := New(Config{Dir: dir, Fresh: time.Hour})
	obs := rt.Observe(t0, []host.Bound{bound(10, "A", t0.Add(-time.Hour)), bound(11, "", t0.Add(-time.Hour))})
	if obs[0].States[0].State != "working" || obs[1].States[0].State != "waiting" {
		t.Fatalf("%+v", obs)
	}
}

func TestANewerProcessTakesASharedSessionAndOlderOnesGetNothing(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "S.json", file("S", 0, "working", t0))
	rt := New(Config{Dir: dir, Fresh: time.Hour})
	obs := rt.Observe(t0, []host.Bound{bound(20, "S", t0.Add(-time.Hour)), bound(19, "S", t0.Add(-2*time.Hour))})
	if len(obs[0].States) != 1 || len(obs[1].States) != 0 {
		t.Fatalf("%+v", obs)
	}
}

func TestAFileOlderThanTheProcessBelongsToAPreviousPIDOwner(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "S.json", file("S", 10, "approval", t0.Add(-time.Hour)))
	rt := New(Config{Dir: dir, Fresh: 24 * time.Hour})
	if o := rt.Observe(t0, []host.Bound{bound(10, "", t0)})[0]; len(o.States) != 0 {
		t.Fatalf("%+v", o)
	}
}

func TestInvalidFilesAreIgnored(t *testing.T) {
	dir := t.TempDir()
	put(t, dir, "wrongname.json", file("S", 10, "working", t0))
	put(t, dir, "S.json", `{"schema":2,"session_id":"S","pid":10,"state":"working","observed_at":"2026-10-27T01:00:00Z"}`)
	put(t, dir, "T.json", file("T", 10, "exploding", t0))
	put(t, dir, "U.json", `{"schema":1,"session_id":"U","pid":10,"state":"working","observed_at":"yesterday"}`)
	put(t, dir, "V.json", "not json")
	put(t, dir, "W.json", file("W", 10, "working", t0)+string(make([]byte, maxBytes)))
	put(t, dir, "X.txt", file("X", 10, "working", t0))
	if err := os.Symlink(filepath.Join(dir, "wrongname.json"), filepath.Join(dir, "L.json")); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "F.json")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	rt := New(Config{Dir: dir, Fresh: time.Hour})
	if o := rt.Observe(t0, []host.Bound{bound(10, "S", t0.Add(-time.Hour))})[0]; len(o.States) != 0 {
		t.Fatalf("an invalid file produced %+v", o)
	}
}

func TestMissingDirectoryAndNoSessionsOfItsOwn(t *testing.T) {
	rt := New(Config{Dir: filepath.Join(t.TempDir(), "absent"), Fresh: time.Hour})
	if o := rt.Observe(t0, []host.Bound{bound(1, "S", t0)}); len(o) != 1 || len(o[0].States) != 0 {
		t.Fatalf("%+v", o)
	}
	if rt.Sessions(t0, nil) != nil || rt.Name() != "hookfile" {
		t.Fatal("hookfile discovers no processes")
	}
}
