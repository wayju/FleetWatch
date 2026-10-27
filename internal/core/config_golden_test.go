package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"encoding/json"
	"github.com/wayju/fleetwatch/internal/config"
	"net/http"
	"net/http/httptest"
)

// fakeEnv is a process with the given variables and no config files beyond
// the ones named.
func fakeEnv(vars map[string]string) config.Env {
	return config.Env{
		Get:         func(k string) string { return vars[k] },
		UserHomeDir: func() (string, error) { return "/nonexistent-home", nil },
		Hostname:    func() (string, error) { return "testhost", nil },
		ReadFile:    os.ReadFile,
		UID:         1000,
	}
}

// procWorld is a host without the optional pool runtime: no pool slots, no hook files, no
// HUD projection. Sessions bind by the session file or a guess, and state
// comes from the transcript alone.
func procWorld(t *testing.T) (home, procRoot string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "g")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	home, procRoot = filepath.Join(dir, "home"), filepath.Join(dir, "proc")
	write(t, filepath.Join(procRoot, "stat"), fmt.Sprintf("cpu 0\nbtime %d\n", t0.Add(-time.Hour).Unix()))
	proj := filepath.Join(home, ".claude", "projects", "-home-src-app")
	write(t, filepath.Join(proj, "aaaaaaaa-1111-2222-3333-444444444444.jsonl"), userText(0, "work")+assistantEnd(9, "Waiting on you."))
	write(t, filepath.Join(proj, "bbbbbbbb-1111-2222-3333-444444444444.jsonl"), userText(10, "tidy")+assistantTool(20, "x1", "Bash", map[string]any{"command": "npm test"}))
	write(t, filepath.Join(home, ".claude", "sessions", "100.json"), `{"sessionId":"aaaaaaaa-1111-2222-3333-444444444444","name":"alpha"}`)
	cg := "0::/user.slice/session-3.scope\n"
	fakeProc(t, procRoot, 100, 1, "claude", "claude", cg, 3600*100)
	fakeProc(t, procRoot, 200, 1, "claude", "claude", cg, 3600*100)
	fakeProc(t, procRoot, 300, 1, "claude", "claude", cg, 3600*100)
	return home, procRoot
}

func TestZeroConfigProc(t *testing.T) {
	home, procRoot := procWorld(t)
	res, err := config.Resolve(fakeEnv(map[string]string{
		"FLEET_HOME": home, "FLEET_PROC_ROOT": procRoot, "FLEET_HOST": "host-a", "FLEET_RESCAN": "1s", "FLEET_PRICES_FILE": testPricesFile(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if p := res.Profile(); fmt.Sprint(p["runtimes"]) != "[proc]" || p["config"] != "default" {
		t.Fatalf("profile %v", p)
	}
	col := collectorFrom(res, nil, nil)
	snap := collectSettled(t, col, t0.Add(30*time.Second))
	by := map[string]SessionView{}
	for _, s := range snap.Sessions {
		by[s.Short] = s
	}
	// Equal start times visit the higher pid first, so pid 300 takes the guess.
	a, b, c := by["aaaaaaaa"], by["bbbbbbbb"], by["pid-200"]
	if a.Link != "session-file" || a.State != "waiting" || a.StateSource != "transcript" || a.Runtime != "proc" || a.Slot != "" || a.Pane != "" {
		t.Errorf("a: %+v", a)
	}
	if b.Link != "guess" || b.PID != 300 || b.State != "working" || b.Runtime != "proc" {
		t.Errorf("b: %+v", b)
	}
	if c.State != "starting" || c.StateSource != "process" || c.Runtime != "proc" {
		t.Errorf("c: %+v", c)
	}
	out, err := marshalNormalised(snap, filepath.Dir(home))
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "v1-proc-default.json", out)
}

func TestHealthReportsTheEffectiveProfile(t *testing.T) {
	res, err := config.Resolve(fakeEnv(nil))
	if err != nil {
		t.Fatal(err)
	}
	h := &pollHealth{profile: res.Profile()}
	h.ok(time.Now(), 0)
	srv := httptest.NewServer(newMux(NewHub(), h, time.Second, nil))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Profile struct {
			Runtimes []string `json:"runtimes"`
			Agents   []string `json:"agents"`
			Config   string   `json:"config"`
		} `json:"profile"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(body.Profile.Runtimes) != "[proc]" || fmt.Sprint(body.Profile.Agents) != "[claudecode]" || body.Profile.Config != "default" {
		t.Fatalf("%+v", body.Profile)
	}
}
