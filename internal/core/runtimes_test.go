package core

import (
	"github.com/wayju/fleetwatch/internal/host"
	"testing"
	"time"
)

type fakeRuntime struct {
	name string
	ss   []host.Session
}

func (f fakeRuntime) Name() string { return f.name }
func (f fakeRuntime) Sessions(time.Time, host.Claimer) []host.Session {
	return f.ss
}

// A runtime that places a session owns the runtime field over the generic proc
// scanner, whichever is configured first. Location rides along, and
// the process facts the scanner knows (start time) are kept.
func TestPlacingRuntimeOwnsRuntimeInEitherOrder(t *testing.T) {
	start := time.Unix(1000, 0)
	procRT := fakeRuntime{"proc", []host.Session{{Runtime: "proc", Agent: "claudecode", PID: 7, PPID: 1, Start: start}}}
	for _, placer := range []string{"tmux", "herdr"} {
		placed := fakeRuntime{placer, []host.Session{{Runtime: placer, Agent: "claudecode", PID: 7, Address: "%3", Location: "main:2.0"}}}
		for name, order := range map[string][]host.Runtime{"proc first": {procRT, placed}, "placer first": {placed, procRT}} {
			got := newRuntimeSet(order...).processes(start, nil)
			if len(got) != 1 {
				t.Fatalf("%s/%s: %d procs", placer, name, len(got))
			}
			p := got[0]
			if p.Runtime != placer || p.Address != "%3" || p.Location != "main:2.0" || !p.Start.Equal(start) {
				t.Errorf("%s/%s: %+v", placer, name, p)
			}
		}
	}
}

func TestFirstNonGenericRuntimeKeepsOwnership(t *testing.T) {
	a := fakeRuntime{"pool", []host.Session{{Runtime: "pool", PID: 7, Label: "work"}}}
	b := fakeRuntime{"tmux", []host.Session{{Runtime: "tmux", PID: 7, Address: "%1", Location: "s:1.0"}}}
	p := newRuntimeSet(a, b).processes(time.Now(), nil)[0]
	if p.Runtime != "pool" || p.Slot != "work" || p.Location != "s:1.0" {
		t.Errorf("%+v", p)
	}
}
