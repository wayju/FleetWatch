package core

import (
	"context"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// pushRuntime is a runtime that can also push (the herdr shape).
type pushRuntime struct{ started chan func() }

func (pushRuntime) Name() string                                    { return "push" }
func (pushRuntime) Sessions(time.Time, host.Claimer) []host.Session { return nil }
func (p pushRuntime) Watch(ctx context.Context, poke func()) error {
	p.started <- poke
	<-ctx.Done()
	return ctx.Err()
}

func TestWatchersReceiveThePokeAndStopWithTheContext(t *testing.T) {
	p := pushRuntime{started: make(chan func(), 1)}
	set := newRuntimeSet(p)
	if len(set.watchers) != 1 {
		t.Fatalf("watchers %v", set.watchers)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fired := make(chan struct{}, 1)
	set.watch(ctx, func() { fired <- struct{}{} })
	select {
	case poke := <-p.started:
		poke()
	case <-time.After(time.Second):
		t.Fatal("watcher not started")
	}
	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("poke not delivered")
	}
}

func TestProcOnlyRuntimesStartNoWatcher(t *testing.T) {
	if n := len(newRuntimeSet().watchers); n != 0 {
		t.Fatal(n)
	}
}
