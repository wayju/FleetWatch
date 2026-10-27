package core

import (
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// Proc is an agent process as the core uses it. Slot is the runtime's label
// for it (a pool slot of an optional runtime); ConfigDir is "" for the agent's default.
type Proc struct {
	PID       int
	PPID      int
	Start     time.Time
	Slot      string
	Cmdline   string
	Runtime   string
	Agent     string
	ConfigDir string
	Address   string
	Location  string // tmux session:window.pane, separate from Slot
}

func procOf(s host.Session) Proc {
	return Proc{PID: s.PID, PPID: s.PPID, Start: s.Start, Slot: s.Label, Cmdline: s.Cmdline,
		Runtime: s.Runtime, Agent: s.Agent, ConfigDir: s.ConfigDir, Address: s.Address, Location: s.Location}
}

func (p Proc) session() host.Session {
	return host.Session{Runtime: p.Runtime, Agent: p.Agent, PID: p.PID, PPID: p.PPID, Start: p.Start, Cmdline: p.Cmdline,
		Label: p.Slot, ConfigDir: p.ConfigDir, Address: p.Address, Location: p.Location}
}
