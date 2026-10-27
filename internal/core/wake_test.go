package core

import (
	"testing"
	"time"
)

// A wake means the session acts again without the user, so the board reports it
// as working with the wake's reason, and keeps next_wake.
func TestWakeFlipsWaitingAndIdleToWorking(t *testing.T) {
	for _, st := range []string{"waiting", "idle", "working", "approval"} {
		v := SessionView{State: st, StateReason: "old", NextWake: &NextWake{Kind: "timer", Reason: "check CI"}}
		wakeIsWork(&v)
		want := "working"
		if st == "approval" {
			want = "approval"
		}
		if v.State != want || (want == "working" && st != "working" && v.StateReason != "check CI") || v.NextWake == nil {
			t.Fatalf("%s -> %+v", st, v)
		}
	}
	v := SessionView{State: "waiting", StateReason: "old"}
	wakeIsWork(&v)
	if v.State != "waiting" || v.StateReason != "old" {
		t.Fatalf("no wake must leave state: %+v", v)
	}
}

func wantTimer(t *testing.T, w *NextWake, sec int) {
	t.Helper()
	if w == nil || w.Kind != "timer" || w.At != t0.Add(time.Duration(sec)*time.Second).Unix() {
		t.Fatalf("want timer at +%ds, got %+v", sec, w)
	}
}
