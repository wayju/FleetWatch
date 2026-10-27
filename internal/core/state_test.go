package core

import (
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

func sig(state string, rank int, at, fresh time.Time) host.StateSignal {
	return host.StateSignal{State: state, Rank: rank, ObservedAt: at, FreshUntil: fresh}
}

func TestMergeStatePicksHighestRank(t *testing.T) {
	now := t0.Add(time.Minute)
	floor := sig("waiting", host.RankTranscript, t0, time.Time{})
	hook := sig("working", host.RankHook, t0.Add(5*time.Second), time.Time{})
	proj := sig("idle", host.RankProjection, t0.Add(9*time.Second), time.Time{})
	got, since := mergeState(now, floor, []host.StateSignal{proj, hook})
	if got.State != "working" || !since.Equal(hook.ObservedAt) {
		t.Fatalf("got %+v since %v", got, since)
	}
	got, _ = mergeState(now, floor, []host.StateSignal{hook, proj})
	if got.State != "working" {
		t.Fatalf("order must not matter: %+v", got)
	}
}

func TestMergeStateTieGoesToNewerObservation(t *testing.T) {
	now := t0.Add(time.Minute)
	floor := sig("waiting", host.RankTranscript, t0, time.Time{})
	a := sig("working", host.RankPane, t0.Add(1*time.Second), time.Time{})
	b := sig("idle", host.RankPane, t0.Add(2*time.Second), time.Time{})
	if got, _ := mergeState(now, floor, []host.StateSignal{a, b}); got.State != "idle" {
		t.Fatalf("got %+v", got)
	}
	if got, _ := mergeState(now, floor, []host.StateSignal{b, a}); got.State != "idle" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeStateSkipsExpiredSignals(t *testing.T) {
	now := t0.Add(time.Minute)
	floor := sig("waiting", host.RankTranscript, t0, time.Time{})
	stale := sig("working", host.RankProjection, t0.Add(time.Second), now.Add(-time.Second))
	if got, _ := mergeState(now, floor, []host.StateSignal{stale}); got.State != "waiting" {
		t.Fatalf("expired signal won: %+v", got)
	}
	live := sig("working", host.RankProjection, t0.Add(time.Second), now.Add(time.Second))
	if got, _ := mergeState(now, floor, []host.StateSignal{live}); got.State != "working" {
		t.Fatalf("fresh signal lost: %+v", got)
	}
}

func TestMergeStateZeroObservedAtFallsBackToFloorSince(t *testing.T) {
	floor := sig("waiting", host.RankTranscript, t0, time.Time{})
	proj := sig("working", host.RankProjection, time.Time{}, time.Time{})
	got, since := mergeState(t0, floor, []host.StateSignal{proj})
	if got.State != "working" || !since.Equal(t0) {
		t.Fatalf("got %+v since %v", got, since)
	}
}

func TestMergeStateEmptyReturnsFloor(t *testing.T) {
	floor := sig("starting", host.RankProcess, t0, time.Time{})
	got, since := mergeState(t0, floor, nil)
	if got != floor || !since.Equal(t0) {
		t.Fatalf("got %+v since %v", got, since)
	}
}
