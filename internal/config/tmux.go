package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/wayju/fleetwatch/internal/host/tmux"
	"github.com/wayju/fleetwatch/internal/registry"
)

// tmuxDoc is the tmux runtime's section (see ADR-002). scrape is off by default: it
// reads each agent's screen every poll, so a user opts in and names the rules.
type tmuxDoc struct {
	Type        string   `json:"type"`
	ProcRoot    string   `json:"proc_root"`
	Binary      string   `json:"binary"`
	Sockets     []string `json:"sockets"`
	Timeout     string   `json:"timeout"`
	Scrape      bool     `json:"scrape"`
	ScrapeLines *int     `json:"scrape_lines"`
	Rules       string   `json:"rules"`
}

func decodeTmux(res registry.Resolver, raw json.RawMessage, i int) (RuntimeSpec, error) {
	var d tmuxDoc
	if err := registry.Strict(raw, &d); err != nil {
		return RuntimeSpec{}, err
	}
	k := "runtimes[" + strconv.Itoa(i) + "]."
	s := RuntimeSpec{Type: "tmux", Scrape: d.Scrape, ScrapeLines: 20, TmuxTimeout: 2 * time.Second}
	s.ProcRoot = res.Pick(k+"proc_root", "FLEET_PROC_ROOT", d.ProcRoot, "/proc")
	s.Binary = res.Pick(k+"binary", "", d.Binary, "tmux")
	for _, p := range d.Sockets {
		s.Sockets = append(s.Sockets, res.Expand(p))
	}
	if d.Timeout != "" {
		t, err := time.ParseDuration(d.Timeout)
		if err != nil || t <= 0 {
			return s, fmt.Errorf("timeout must be a positive duration like \"2s\", got %q", d.Timeout)
		}
		s.TmuxTimeout = t
	}
	if d.ScrapeLines != nil {
		if *d.ScrapeLines < 1 || *d.ScrapeLines > 1000 {
			return s, fmt.Errorf("scrape_lines must be between 1 and 1000, got %d", *d.ScrapeLines)
		}
		s.ScrapeLines = *d.ScrapeLines
	}
	if d.Rules == "" {
		if d.Scrape {
			return s, fmt.Errorf("scrape is on but rules names no file")
		}
		return s, nil
	}
	s.RulesFile = res.Pick(k+"rules", "", d.Rules, "")
	data, err := res.ReadFile(s.RulesFile)
	if err != nil {
		return s, fmt.Errorf("rules: %v", err)
	}
	if s.Rules, err = tmux.ParseRules(data); err != nil {
		return s, fmt.Errorf("rules %s: %v", s.RulesFile, err)
	}
	return s, nil
}
