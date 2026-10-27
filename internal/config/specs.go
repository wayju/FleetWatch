// specs.go: the resolved settings of the built-in adapters. Adapters outside
// this package (an optional one registered from cmd/fleetwatch) bring their own
// settings type and satisfy the same registry interfaces.
package config

import (
	"log"
	"strconv"
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/agent/claudecode"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/host/herdr"
	"github.com/wayju/fleetwatch/internal/host/hookfile"
	"github.com/wayju/fleetwatch/internal/host/proc"
	"github.com/wayju/fleetwatch/internal/host/tmux"
	"github.com/wayju/fleetwatch/internal/registry"
)

// RuntimeEntry is one configured runtime: the adapter name and its settings.
type RuntimeEntry struct {
	Type     string
	Settings registry.RuntimeSettings
}

// AgentEntry is one configured agent: the adapter name and its settings.
type AgentEntry struct {
	Type     string
	Settings registry.AgentSettings
}

// RuntimeSpec is a built-in runtime's settings, with its paths resolved. One
// struct serves proc, tmux, herdr and hookfile; the fields each uses are noted.
type RuntimeSpec struct {
	Type     string
	ProcRoot string // proc, tmux

	Binary      string // tmux
	Sockets     []string
	TmuxTimeout time.Duration
	Scrape      bool
	ScrapeLines int
	RulesFile   string
	Rules       tmux.Rules
	Socket      string // herdr
	Rank        int    // herdr: rank of a screen-detected state

	HookfileDir   string        // hookfile
	HookfileFresh time.Duration // hookfile
}

// Build constructs the runtime this spec describes.
func (s RuntimeSpec) Build(c registry.BuildCtx) host.Runtime {
	switch s.Type {
	case "proc":
		return proc.Runtime{Root: s.ProcRoot}
	case "tmux":
		return tmux.New(tmux.Config{ProcRoot: s.ProcRoot, Binary: s.Binary, Sockets: s.Sockets, Timeout: s.TmuxTimeout,
			Scrape: s.Scrape, ScrapeLines: s.ScrapeLines, Rules: s.Rules, Poll: c.Poll})
	case "herdr":
		return herdr.New(herdr.Config{Socket: s.Socket, Rank: s.Rank, Logf: log.Printf})
	case "hookfile":
		return hookfile.New(hookfile.Config{Dir: s.HookfileDir, Fresh: s.HookfileFresh})
	}
	return nil
}

// Value renders a setting for the startup report.
func (s RuntimeSpec) Value(key string) (string, bool) {
	switch key {
	case "proc_root":
		return s.ProcRoot, true
	case "socket":
		return s.Socket, true
	case "rank":
		return strconv.Itoa(s.Rank), true
	case "dir":
		return s.HookfileDir, true
	case "fresh":
		return s.HookfileFresh.String(), true
	case "binary":
		return s.Binary, true
	case "rules":
		return s.RulesFile, true
	}
	return "", false
}

// AgentSpec is a built-in agent's settings, with its paths resolved.
type AgentSpec struct {
	Type       string
	ConfigDirs []string // claudecode; the first is the default
}

// Build constructs the agent this spec describes.
func (s AgentSpec) Build(c registry.BuildCtx) agent.Agent {
	var dirs []host.ConfigDir
	for _, p := range s.ConfigDirs {
		dirs = append(dirs, host.ConfigDir{Path: p})
	}
	return claudecode.New(claudecode.Config{Dirs: dirs, ExtraDirs: c.ExtraDirs, Window: c.Window, RescanEvery: c.Rescan,
		PricesFile: c.PricesFile, Models: c.Models})
}

// Value renders a setting for the startup report.
func (s AgentSpec) Value(key string) (string, bool) {
	if key == "config_dirs" {
		return joinComma(s.ConfigDirs), true
	}
	return "", false
}
