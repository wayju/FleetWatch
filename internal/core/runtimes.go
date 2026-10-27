package core

import (
	"context"
	"sort"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// runtimeSet is the configured runtimes, with each optional capability looked
// up once. The core talks to runtimes only through this.
type runtimeSet struct {
	all       []host.Runtime
	observers []host.Observer
	dirs      []host.ConfigDirSource
	activity  []host.ActivitySource
	context   []host.ContextSource
	watchers  []host.Watcher
}

func newRuntimeSet(rts ...host.Runtime) runtimeSet {
	s := runtimeSet{all: rts}
	for _, r := range rts {
		if o, ok := r.(host.Observer); ok {
			s.observers = append(s.observers, o)
		}
		if d, ok := r.(host.ConfigDirSource); ok {
			s.dirs = append(s.dirs, d)
		}
		if a, ok := r.(host.ActivitySource); ok {
			s.activity = append(s.activity, a)
		}
		if c, ok := r.(host.ContextSource); ok {
			s.context = append(s.context, c)
		}
		if w, ok := r.(host.Watcher); ok {
			s.watchers = append(s.watchers, w)
		}
	}
	return s
}

// processes lists the agent processes, newest first (pid descending on a tie).
// A pid belongs to the first runtime that reports it; later runtimes only fill
// the label, config dir and address it left empty.
func (s runtimeSet) processes(now time.Time, claim host.Claimer) []Proc {
	byPID := map[int]*host.Session{}
	var order []int
	for _, r := range s.all {
		for _, rs := range r.Sessions(now, claim) {
			rs := rs
			prev, ok := byPID[rs.PID]
			if !ok {
				byPID[rs.PID] = &rs
				order = append(order, rs.PID)
				continue
			}
			mergeSession(prev, rs)
		}
	}
	out := make([]Proc, 0, len(order))
	for _, pid := range order {
		out = append(out, procOf(*byPID[pid]))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Start.Equal(out[j].Start) {
			return out[i].Start.After(out[j].Start)
		}
		return out[i].PID > out[j].PID
	})
	return out
}

// genericRuntime is the process scanner that reports every agent process
// without placing it anywhere.
const genericRuntime = "proc"

// mergeSession folds a later runtime's report of the same pid into the first.
// The first reporter owns the runtime field, except that a runtime which
// actually places the session (tmux, herdr) displaces the generic scanner
// whichever order they are configured in. Later runtimes only fill
// what is still empty.
func mergeSession(prev *host.Session, rs host.Session) {
	if prev.Runtime == genericRuntime && rs.Runtime != genericRuntime {
		prev.Runtime = rs.Runtime
	}
	if prev.Start.IsZero() {
		prev.Start = rs.Start // herdr cannot know a start time; proc can
	}
	if prev.PPID == 0 {
		prev.PPID = rs.PPID
	}
	if prev.Cmdline == "" {
		prev.Cmdline = rs.Cmdline
	}
	if prev.Label == "" {
		prev.Label = rs.Label
	}
	if prev.ConfigDir == "" {
		prev.ConfigDir = rs.ConfigDir
	}
	if prev.Address == "" {
		prev.Address = rs.Address
	}
	if prev.Location == "" {
		prev.Location = rs.Location
	}
}

// observe asks every observer about every bound session and merges the
// answers in runtime order: the first address and the first context or task
// scan win, state signals and servers accumulate, and the first session-id
// hint stands.
func (s runtimeSet) observe(now time.Time, bs []host.Bound) []host.Observation {
	out := make([]host.Observation, len(bs))
	for _, o := range s.observers {
		for i, ob := range o.Observe(now, bs) {
			m := &out[i]
			if m.SessionID == "" {
				m.SessionID, m.How = ob.SessionID, ob.How
			}
			if m.Address == "" {
				m.Address = ob.Address
			}
			if m.Context == nil {
				m.Context = ob.Context
			}
			if m.Tasks == nil {
				m.Tasks = ob.Tasks
			}
			m.States = append(m.States, ob.States...)
			m.Servers = append(m.Servers, ob.Servers...)
		}
	}
	return out
}

// configDirs are the extra transcript roots the runtimes know about.
func (s runtimeSet) configDirs() []host.ConfigDir {
	var out []host.ConfigDir
	for _, d := range s.dirs {
		out = append(out, d.ConfigDirs()...)
	}
	return out
}

// subagentTouched is when a subagent hook last touched the session.
func (s runtimeSet) subagentTouched(sessionID, address string) (time.Time, bool) {
	for _, a := range s.activity {
		if at, ok := a.SubagentTouched(sessionID, address); ok {
			return at, true
		}
	}
	return time.Time{}, false
}

// contextCache is the statusline's cached context fill for an address.
func (s runtimeSet) contextCache(address string) (float64, bool) {
	for _, c := range s.context {
		if pct, ok := c.ContextCache(address); ok {
			return pct, true
		}
	}
	return 0, false
}

// watch starts every push-capable runtime; each calls poke to wake the poll
// loop early. They stop when ctx ends.
func (s runtimeSet) watch(ctx context.Context, poke func()) {
	for _, w := range s.watchers {
		go func(w host.Watcher) { _ = w.Watch(ctx, poke) }(w)
	}
}
