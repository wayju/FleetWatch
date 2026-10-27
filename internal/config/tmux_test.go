package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const rulesPath = "/etc/fleet/rules.json"
const rulesBody = `{"claudecode":[{"state":"working","regex":"spin"}]}`

func tmuxFile(runtime string) (map[string]string, map[string]string) {
	vars, files := withFile(`{"version":1,"runtimes":[{"type":"proc"},` + runtime + `],"agents":[{"type":"claudecode"}]}`)
	files[rulesPath] = rulesBody
	return vars, files
}

func TestTmuxDefaults(t *testing.T) {
	vars, files := tmuxFile(`{"type":"tmux"}`)
	r := mustResolve(t, vars, files)
	s := spec(r.Runtimes[1])
	if s.Type != "tmux" || s.Binary != "tmux" || len(s.Sockets) != 0 || s.Scrape || s.ScrapeLines != 20 ||
		s.TmuxTimeout != 2*time.Second || s.ProcRoot != "/proc" || s.Rules != nil {
		t.Fatalf("%+v", s)
	}
	if rts, _ := r.Build(); len(rts) != 2 || rts[1].Name() != "tmux" {
		t.Fatalf("%v", rts)
	}
}

func TestTmuxValidSection(t *testing.T) {
	vars, files := tmuxFile(`{"type":"tmux","binary":"/usr/bin/tmux","sockets":["/tmp/tmux-1000/default","~/s"],"timeout":"500ms","scrape":true,"scrape_lines":40,"rules":"` + rulesPath + `"}`)
	s := spec(mustResolve(t, vars, files).Runtimes[1])
	if s.Binary != "/usr/bin/tmux" || len(s.Sockets) != 2 || s.Sockets[1] != "/home/proc/s" || s.TmuxTimeout != 500*time.Millisecond ||
		!s.Scrape || s.ScrapeLines != 40 || len(s.Rules["claudecode"]) != 1 {
		t.Fatalf("%+v", s)
	}
}

func TestTmuxRejectsBadInput(t *testing.T) {
	for want, rt := range map[string]string{
		"unknown field":     `{"type":"tmux","scrap":true}`,
		"scrape is on":      `{"type":"tmux","scrape":true}`,
		"positive duration": `{"type":"tmux","timeout":"0s"}`,
		"scrape_lines":      `{"type":"tmux","scrape_lines":0}`,
		"rules":             `{"type":"tmux","rules":"/missing.json"}`,
		"unknown state":     `{"type":"tmux","rules":"/bad.json"}`,
	} {
		vars, files := tmuxFile(rt)
		files["/bad.json"] = `{"claudecode":[{"state":"asleep","regex":"x"}]}`
		failsWith(t, vars, files, want)
	}
}

func TestZeroConfigAddsTmuxOnlyWhenOnPath(t *testing.T) {
	env := envOf(nil, nil)
	env.LookPath = func(string) (string, error) { return "/usr/bin/tmux", nil }
	r, err := Resolve(env)
	if err != nil || len(r.Runtimes) != 2 || r.Runtimes[0].Type != "proc" || r.Runtimes[1].Type != "tmux" {
		t.Fatalf("%+v %v", r.Runtimes, err)
	}
	env.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	if r, _ = Resolve(env); len(r.Runtimes) != 1 {
		t.Fatalf("%+v", r.Runtimes)
	}
	if strings.Contains(strings.Join(r.Describe(), "\n"), "tmux") {
		t.Fatal("describe names tmux without it")
	}
}
