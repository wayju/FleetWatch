package tmux

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/host"
)

// Rule says that a screen matching Regex means the agent is in State. Rules
// live in a file named in config, never in code, because the wording of an
// agent's prompts changes with its releases.
type Rule struct {
	State  string `json:"state"`
	Reason string `json:"reason"`
	Regex  string `json:"regex"`
	re     *regexp.Regexp
}

// Rules maps an agent type to its rules, tried in order; the first match wins.
type Rules map[string][]Rule

var states = map[string]bool{"working": true, "waiting": true, "approval": true, "question": true, "idle": true}

// ParseRules reads a rules file strictly: unknown keys, unknown states and
// patterns that do not compile are errors, so a typo cannot silently turn a
// rule off.
func ParseRules(data []byte) (Rules, error) {
	var rs Rules
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rs); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("unexpected data after the JSON document")
	}
	for agent, list := range rs {
		for i := range list {
			r := &list[i]
			if !states[r.State] {
				return nil, fmt.Errorf("%s[%d]: unknown state %q", agent, i, r.State)
			}
			re, err := regexp.Compile(r.Regex)
			if err != nil || r.Regex == "" {
				return nil, fmt.Errorf("%s[%d]: bad regex %q", agent, i, r.Regex)
			}
			r.re = re
		}
	}
	return rs, nil
}

// match is the first rule of the agent that the screen satisfies.
func (rs Rules) match(agent, screen string) (Rule, bool) {
	for _, r := range rs[agent] {
		if r.re != nil && r.re.MatchString(screen) {
			return r, true
		}
	}
	return Rule{}, false
}

// Agents lists the agents that have rules, for diagnostics.
func (rs Rules) Agents() []string {
	var out []string
	for a := range rs {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// trimScreen is the pane's last n lines with trailing blanks removed, so a rule
// can anchor to the bottom of the screen and ignore scrollback.
func trimScreen(raw string) string {
	lines := strings.Split(raw, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// Observe reads each tracked pane's screen and reports the state a rule gives.
// The signal is a pane scrape (RankPane) that stays fresh for two polls, and
// carries no ObservedAt, so a long wait still turns idle on the transcript's
// clock rather than restarting at every scrape.
func (r *Runtime) Observe(now time.Time, bs []host.Bound) []host.Observation {
	out := make([]host.Observation, len(bs))
	r.mu.Lock()
	panes := r.byPID
	r.mu.Unlock()
	for i, b := range bs {
		p, ok := panes[b.PID]
		if !ok {
			continue
		}
		out[i].Address = p.id
		if !r.cfg.Scrape || len(r.cfg.Rules[b.Agent]) == 0 {
			continue
		}
		raw, err := r.call(p.socket, "capture-pane", "-p", "-J", "-S", "-"+strconv.Itoa(r.cfg.ScrapeLines), "-t", p.id)
		if err != nil {
			continue
		}
		if rule, ok := r.cfg.Rules.match(b.Agent, trimScreen(string(raw))); ok {
			out[i].States = []host.StateSignal{{State: rule.State, Reason: rule.Reason, Source: "pane",
				Rank: host.RankPane, FreshUntil: now.Add(2 * r.cfg.Poll)}}
		}
	}
	return out
}
