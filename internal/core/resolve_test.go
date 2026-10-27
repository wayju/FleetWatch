package core

import (
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
)

// fakeSession is an agent session with canned answers; it records what the
// core asked its NextWake.
type fakeSession struct {
	agent.Session
	floor host.StateSignal
	wake  *board.NextWake // returned for waiting and idle, as a real session does
	asked struct {
		state         string
		async, parked int
		tasks         *host.TaskCap
	}
	digest string
}

func (f *fakeSession) State(time.Time) host.StateSignal { return f.floor }
func (f *fakeSession) NextWake(state string, async, parked int, tasks *host.TaskCap, _ func(string) string) *board.NextWake {
	f.asked.state, f.asked.async, f.asked.parked, f.asked.tasks = state, async, parked, tasks
	if state != "waiting" && state != "idle" {
		return nil
	}
	return f.wake
}
func (f *fakeSession) Digest(board.SessionView) string { return f.digest }

func transcriptFloor(state string, at time.Time) host.StateSignal {
	return host.StateSignal{State: state, Source: "transcript", Rank: host.RankTranscript, ObservedAt: at}
}

// hookLink is a link whose runtime reported a hook signal.
func hookLink(state string, at time.Time) *liveLink {
	return &liveLink{obs: host.Observation{States: []host.StateSignal{
		{State: state, Reason: "none", Source: "hook", Rank: host.RankHook, ObservedAt: at}}}}
}
