package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// envOf is a fake process: variables, files by path, and the process's own
// home and host.
func envOf(vars map[string]string, files map[string]string) Env {
	return Env{
		Get:         func(k string) string { return vars[k] },
		UserHomeDir: func() (string, error) { return "/home/proc", nil },
		Hostname:    func() (string, error) { return "box", nil },
		UID:         1000,
		ReadFile: func(p string) ([]byte, error) {
			if s, ok := files[p]; ok {
				return []byte(s), nil
			}
			return nil, os.ErrNotExist
		},
	}
}

func mustResolve(t *testing.T, vars, files map[string]string) *Resolved {
	t.Helper()
	r, err := Resolve(envOf(vars, files))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func failsWith(t *testing.T, vars, files map[string]string, want string) {
	t.Helper()
	_, err := Resolve(envOf(vars, files))
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("want an error containing %q, got %v", want, err)
	}
}

const cfgPath = "/etc/fleet/config.json"

func withFile(body string) (map[string]string, map[string]string) {
	return map[string]string{"FLEET_CONFIG": cfgPath}, map[string]string{cfgPath: body}
}

func TestZeroConfigIsProcAndClaudecode(t *testing.T) {
	r := mustResolve(t, nil, nil)
	if r.Path != "default" || len(r.Runtimes) != 1 || r.Runtimes[0].Type != "proc" || spec(r.Runtimes[0]).ProcRoot != "/proc" {
		t.Fatalf("runtimes %+v path %s", r.Runtimes, r.Path)
	}
	if len(r.Agents) != 1 || r.Agents[0].Type != "claudecode" || aspec(r.Agents[0]).ConfigDirs[0] != "/home/proc/.claude" {
		t.Fatalf("agents %+v", r.Agents)
	}
	if r.Host != "box" || r.Home != "/home/proc" || r.Poll != 2*time.Second || r.Window != 2*time.Hour || r.Recent != 30*time.Minute ||
		r.Rescan != 10*time.Second || !r.ShowText || r.Port != "8909" || r.MaxSubscribers != 32 || len(r.AllowedHosts) != 2 || r.Models.Len() != 0 || r.PricesFile != "" || r.RecapModel != "" {
		t.Fatalf("defaults %+v", r)
	}
	if p := r.Profile(); p["config"] != "default" {
		t.Fatalf("profile %v", p)
	}
}

func TestPrecedenceDefaultThenFileThenEnv(t *testing.T) {
	body := `{"version":1,"host":"filehost","home":"/h","poll":"5s","window":"1h","recent":"10m","rescan":"3s","show_text":false,
		"server":{"port":9000,"allowed_hosts":["A.example:9000"],"max_subscribers":7},
		"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`
	vars, files := withFile(body)
	r := mustResolve(t, vars, files)
	if r.Host != "filehost" || r.Home != "/h" || r.Poll != 5*time.Second || r.Window != time.Hour || r.Recent != 10*time.Minute ||
		r.Rescan != 3*time.Second || r.ShowText || r.Port != "9000" || r.MaxSubscribers != 7 || r.AllowedHosts[0] != "a.example:9000" {
		t.Fatalf("file values %+v", r)
	}
	for _, k := range []string{"host", "home", "poll", "window", "recent", "rescan", "show_text", "port", "allowed_hosts", "max_subscribers"} {
		if r.Sources[k] != "file" {
			t.Errorf("%s from %s, want file", k, r.Sources[k])
		}
	}
	vars["FLEET_HOST"], vars["FLEET_HOME"], vars["FLEET_POLL"], vars["FLEET_WINDOW"] = "envhost", "/e", "1s", "30m"
	vars["FLEET_RECENT"], vars["FLEET_RESCAN"], vars["FLEET_SHOW_TEXT"], vars["FLEET_PORT"] = "5m", "1s", "1", "8000"
	vars["FLEET_ALLOWED_HOSTS"], vars["FLEET_MAX_SUBSCRIBERS"] = "B.example:8000, c.example:8000", "9"
	r = mustResolve(t, vars, files)
	if r.Host != "envhost" || r.Home != "/e" || r.Poll != time.Second || r.Window != 30*time.Minute || r.Recent != 5*time.Minute ||
		r.Rescan != time.Second || !r.ShowText || r.Port != "8000" || r.MaxSubscribers != 9 ||
		len(r.AllowedHosts) != 2 || r.AllowedHosts[0] != "b.example:8000" {
		t.Fatalf("env values %+v", r)
	}
	for k, src := range r.Sources {
		if (k == "runtimes" || k == "agents" || strings.Contains(k, "[")) && src != "file" && src != "default" {
			t.Errorf("%s from %s", k, src)
		}
	}
	if r.Sources["host"] != "env" || r.Sources["max_subscribers"] != "env" {
		t.Errorf("sources %v", r.Sources)
	}
}

func TestInvalidEnvValuesAreIgnored(t *testing.T) {
	vars, files := withFile(`{"version":1,"poll":"5s","runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`)
	vars["FLEET_POLL"], vars["FLEET_MAX_SUBSCRIBERS"], vars["FLEET_SHOW_TEXT"] = "soon", "-3", "maybe"
	r := mustResolve(t, vars, files)
	if r.Poll != 5*time.Second || r.MaxSubscribers != 32 || !r.ShowText || r.Sources["poll"] != "file" {
		t.Fatalf("%+v %v", r, r.Sources)
	}
}

func TestTildeExpandsAgainstFleetHomeNotTheProcessHome(t *testing.T) {
	vars, files := withFile(`{"version":1,"runtimes":[{"type":"hookfile","dir":"~/sessions"}],"agents":[{"type":"claudecode","config_dirs":["~/.claude","~/.claude-extra"]}]}`)
	vars["FLEET_HOME"] = "/mounted/home"
	r := mustResolve(t, vars, files)
	if spec(r.Runtimes[0]).HookfileDir != "/mounted/home/sessions" || aspec(r.Agents[0]).ConfigDirs[1] != "/mounted/home/.claude-extra" {
		t.Fatalf("%+v %+v", spec(r.Runtimes[0]), aspec(r.Agents[0]))
	}
}

func TestBrokenConfigIsAnErrorNeverTheDefault(t *testing.T) {
	good := `"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]`
	cases := map[string]struct{ body, want string }{
		"unknown key":     {`{"version":1,"hoost":"x",` + good + `}`, "unknown field"},
		"unknown adapter": {`{"version":1,"runtimes":[{"type":"zellij"}],"agents":[{"type":"claudecode"}]}`, `unknown type "zellij" (known: herdr, hookfile, proc, tmux)`},
		"unknown agent":   {`{"version":1,"runtimes":[{"type":"proc"}],"agents":[{"type":"codex"}]}`, `unknown type "codex" (known: claudecode)`},
		"adapter key":     {`{"version":1,"runtimes":[{"type":"proc","socket":"x"}],"agents":[{"type":"claudecode"}]}`, "unknown field"},
		"old forge key":   {`{"version":1,"forge":{"x":1},` + good + `}`, "unknown field"},
		"cost key":        {`{"version":1,"cost":{"price":"x"},` + good + `}`, "cost: json: unknown field"},
		"recap key":       {`{"version":1,"recap":{"key":"x"},` + good + `}`, "recap: json: unknown field"},
		"recap every":     {`{"version":1,"recap":{"every":"soon"},` + good + `}`, "recap.every must be a positive duration"},
		"models key":      {`{"version":1,"models":[{"match":"a","context_window":1,"price":2}],` + good + `}`, "models: json: unknown field"},
		"models no match": {`{"version":1,"models":[{"context_window":1}],` + good + `}`, "match is required"},
		"models no size":  {`{"version":1,"models":[{"match":"a"}],` + good + `}`, "context_window must be positive"},
		"models bad glob": {`{"version":1,"models":[{"match":"[","context_window":1}],` + good + `}`, "bad match"},
		"models extended": {`{"version":1,"models":[{"match":"a","context_window":5,"extended_window":5}],` + good + `}`, "extended_window must exceed"},
		"token refused":   {`{"version":1,"fleet_token":"s3cret",` + good + `}`, "unknown field"},
		"nested token":    {`{"version":1,"server":{"token":"s3cret"},` + good + `}`, "unknown field"},
		"bad version":     {`{"version":2,` + good + `}`, "version must be 1"},
		"missing version": {`{` + good + `}`, "version must be 1"},
		"bad duration":    {`{"version":1,"poll":"soon",` + good + `}`, "positive duration"},
		"bad port":        {`{"version":1,"server":{"port":70000},` + good + `}`, "out of range"},
		"empty runtimes":  {`{"version":1,"runtimes":[],"agents":[{"type":"claudecode"}]}`, "at least one"},
		"missing type":    {`{"version":1,"runtimes":[{}],"agents":[{"type":"claudecode"}]}`, "missing"},
		"trailing data":   {`{"version":1,` + good + `} {}`, "unexpected data"},
		"not json":        {`version: 1`, "invalid character"},
	}
	for name, c := range cases {
		vars, files := withFile(c.body)
		t.Run(name, func(t *testing.T) { failsWith(t, vars, files, c.want) })
	}
}

func TestFleetConfigNamingAMissingFileIsAnError(t *testing.T) {
	_, err := Resolve(envOf(map[string]string{"FLEET_CONFIG": "/nope.json"}, nil))
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("got %v", err)
	}
}

func TestDefaultLocationIsReadWhenPresentAndFailsWhenBroken(t *testing.T) {
	xdg := "/xdg/fleet/config.json"
	good := `{"version":1,"host":"fromxdg","runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`
	r := mustResolve(t, map[string]string{"XDG_CONFIG_HOME": "/xdg"}, map[string]string{xdg: good})
	if r.Host != "fromxdg" || r.Path != xdg {
		t.Fatalf("%+v", r)
	}
	home := "/home/proc/.config/fleet/config.json"
	r = mustResolve(t, nil, map[string]string{home: good})
	if r.Path != home {
		t.Fatalf("home config: %s", r.Path)
	}
	failsWith(t, map[string]string{"XDG_CONFIG_HOME": "/xdg"}, map[string]string{xdg: `{"version":1,"oops":1}`}, "unknown field")
}

func TestBuildMakesTheNamedAdapters(t *testing.T) {
	root := t.TempDir()
	vars, files := withFile(`{"version":1,"home":"` + root + `","runtimes":[{"type":"proc","proc_root":"` + filepath.Join(root, "proc") + `"},{"type":"hookfile","dir":"` + filepath.Join(root, "sessions") + `"}],"agents":[{"type":"claudecode"}]}`)
	rts, ags := mustResolve(t, vars, files).Build()
	if len(rts) != 2 || rts[0].Name() != "proc" || rts[1].Name() != "hookfile" || len(ags) != 1 || ags[0].Name() != "claudecode" {
		t.Fatalf("%v %v", rts, ags)
	}
}

func TestDescribeNamesEveryValueAndItsSource(t *testing.T) {
	vars, files := withFile(`{"version":1,"runtimes":[{"type":"hookfile"}],"agents":[{"type":"claudecode"}]}`)
	vars["FLEET_PORT"] = "8000"
	out := strings.Join(mustResolve(t, vars, files).Describe(), "\n")
	for _, want := range []string{"config: " + cfgPath, "port", "env", "8000", "runtimes[0].dir", "default", "/home/proc/.local/state/fleet/sessions", "agents[0].config_dirs"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestSectionsFromFileThenEnv(t *testing.T) {
	body := `{"version":1,"cost":{"prices":"~/prices.json"},
		"models":[{"match":"big-*","context_window":1000,"class":"large"},{"match":"*","context_window":100}],
		"recap":{"url":"https://gw.example","model":"small","every":"2m"},
		"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`
	vars, files := withFile(body)
	r := mustResolve(t, vars, files)
	if r.PricesFile != "/home/proc/prices.json" || r.Models.Len() != 2 || r.Models.Class("big-1") != "large" || r.Models.Window("other", 0) != 100 {
		t.Fatalf("cost/models %+v", r)
	}
	if r.RecapURL != "https://gw.example" || r.RecapModel != "small" || r.RecapEvery != 2*time.Minute {
		t.Fatalf("recap %+v", r)
	}
	for _, k := range []string{"cost.prices", "models", "recap.url", "recap.model", "recap.every"} {
		if r.Sources[k] != "file" {
			t.Errorf("%s from %s, want file", k, r.Sources[k])
		}
	}
	vars["FLEET_PRICES_FILE"] = "/env/p.json"
	vars["FLEET_RECAP_MODEL"], vars["FLEET_RECAP_EVERY"] = "envmodel", "9m"
	r = mustResolve(t, vars, files)
	if r.PricesFile != "/env/p.json" || r.RecapModel != "envmodel" ||
		r.RecapEvery != 9*time.Minute || r.RecapURL != "https://gw.example" {
		t.Fatalf("env over file %+v", r)
	}
	if r.Sources["recap.model"] != "env" {
		t.Errorf("sources %v", r.Sources)
	}
}

func TestSectionsDefaultToNothingConfigured(t *testing.T) {
	r := mustResolve(t, nil, nil)
	if r.RecapEvery != 5*time.Minute || r.Sources["recap.model"] != "default" || r.Sources["models"] != "default" {
		t.Fatalf("%+v", r)
	}
}

// With no allowed_hosts the guard admits only this machine, on the port in
// effect, and names no site-specific host (see ADR-005).
func TestDefaultAllowedHostsFollowThePort(t *testing.T) {
	r := mustResolve(t, map[string]string{"FLEET_PORT": "9100"}, nil)
	if len(r.AllowedHosts) != 2 || r.AllowedHosts[0] != "localhost:9100" || r.AllowedHosts[1] != "127.0.0.1:9100" {
		t.Fatalf("%v", r.AllowedHosts)
	}
}

func TestHookfileDirDefaultsFromXDGStateThenHomeAndFileAndEnvOverride(t *testing.T) {
	body := `{"version":1,"home":"/data/home","runtimes":[{"type":"proc"},{"type":"hookfile"}],"agents":[{"type":"claudecode"}]}`
	vars, files := withFile(body)
	rt := spec(mustResolve(t, vars, files).Runtimes[1])
	if rt.HookfileDir != "/data/home/.local/state/fleet/sessions" || rt.HookfileFresh != time.Hour {
		t.Fatalf("home default: %+v", rt)
	}
	vars["XDG_STATE_HOME"] = "/xdg/state"
	r := mustResolve(t, vars, files)
	if spec(r.Runtimes[1]).HookfileDir != "/xdg/state/fleet/sessions" || r.Sources["runtimes[1].dir"] != "default" {
		t.Fatalf("xdg default: %+v %v", spec(r.Runtimes[1]), r.Sources)
	}
	vars, files = withFile(`{"version":1,"runtimes":[{"type":"hookfile","dir":"~/sessions","fresh":"5m"}],"agents":[{"type":"claudecode"}]}`)
	r = mustResolve(t, vars, files)
	if spec(r.Runtimes[0]).HookfileDir != "/home/proc/sessions" || spec(r.Runtimes[0]).HookfileFresh != 5*time.Minute || r.Sources["runtimes[0].dir"] != "file" {
		t.Fatalf("file: %+v %v", spec(r.Runtimes[0]), r.Sources)
	}
	vars["FLEET_HOOKFILE_DIR"] = "/env/sessions"
	if got := spec(mustResolve(t, vars, files).Runtimes[0]).HookfileDir; got != "/env/sessions" {
		t.Fatalf("env over file: %s", got)
	}
}

func TestHookfileRejectsUnknownKeysAndBadFresh(t *testing.T) {
	tail := `],"agents":[{"type":"claudecode"}]}`
	for body, want := range map[string]string{
		`{"version":1,"runtimes":[{"type":"hookfile","path":"/x"}` + tail:    "unknown field",
		`{"version":1,"runtimes":[{"type":"hookfile","fresh":"soon"}` + tail: "positive duration",
		`{"version":1,"runtimes":[{"type":"hookfile","fresh":"-1m"}` + tail:  "positive duration",
	} {
		vars, files := withFile(body)
		failsWith(t, vars, files, want)
	}
}

func TestBuildMakesTheHookfileRuntime(t *testing.T) {
	vars, files := withFile(`{"version":1,"runtimes":[{"type":"hookfile","dir":"` + t.TempDir() + `"}],"agents":[{"type":"claudecode"}]}`)
	rts, _ := mustResolve(t, vars, files).Build()
	if len(rts) != 1 || rts[0].Name() != "hookfile" {
		t.Fatalf("%v", rts)
	}
}
