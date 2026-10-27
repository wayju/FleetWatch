package core

// The hookfile source against the ranked merge and the board projection
// (see ADR-010): a plugin's state file outranks a scrape, a projection and the
// transcript, and stops winning once it is no longer fresh.

import (
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

func TestNativeOutranksEveryOtherSourceUntilItExpires(t *testing.T) {
	now := t0.Add(time.Minute)
	floor := sig("waiting", host.RankTranscript, t0, time.Time{})
	scrape := sig("working", host.RankPane, t0.Add(9*time.Second), time.Time{})
	hook := sig("working", host.RankHook, t0.Add(9*time.Second), time.Time{})
	proj := sig("waiting", host.RankProjection, t0.Add(9*time.Second), time.Time{})
	native := sig("approval", host.RankNative, t0.Add(2*time.Second), t0.Add(time.Hour))
	if got, _ := mergeState(now, floor, []host.StateSignal{scrape, hook, proj, native}); got.State != "approval" {
		t.Fatalf("native lost to an older, lower-ranked signal: %+v", got)
	}
	expired := native
	expired.FreshUntil = now.Add(-time.Second)
	if got, _ := mergeState(now, floor, []host.StateSignal{scrape, hook, proj, expired}); got.State != "working" || got.Rank != host.RankHook {
		t.Fatalf("a stale file kept winning: %+v", got)
	}
}
