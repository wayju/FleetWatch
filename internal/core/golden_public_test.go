package core

// The public end-to-end golden: the collector driven by a config file naming
// only the portable runtimes (proc and hookfile), over a fabricated /proc tree,
// a git checkout, a transcript and a session-state file. It uses no build tag
// and no site-specific names, so it runs in the exported tree too.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/config"
)

const (
	pubSidA = "aaaaaaaa-1111-2222-3333-444444444444"
	pubSidB = "bbbbbbbb-1111-2222-3333-444444444444"
)

// publicWorld lays out host-a: a checkout of example/widgets, two live agent
// processes, one transcript each, and a state file that places the first in an
// approval prompt. It returns the root the golden normalises to $ROOT and the
// config file's path.
func publicWorld(t *testing.T) (root, cfgPath string) {
	t.Helper()
	root, err := os.MkdirTemp("", "g")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	home, procRoot := filepath.Join(root, "home", "someone"), filepath.Join(root, "proc")
	repo := filepath.Join(home, "widgets")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/7-sprocket\n")
	write(t, filepath.Join(repo, ".git", "refs", "heads", "feature", "7-sprocket"), "0123456789abcdef0123456789abcdef01234567\n")
	write(t, filepath.Join(repo, ".git", "config"), "[remote \"origin\"]\n\turl = https://git.example.test/example/widgets.git\n")
	write(t, filepath.Join(repo, "VERSION"), "1.2.0\n")
	write(t, filepath.Join(home, ".claude.json"), `{"oauthAccount":{"emailAddress":"someone@acme.example","organizationName":"Acme"}}`)

	write(t, filepath.Join(procRoot, "stat"), fmt.Sprintf("cpu 0\nbtime %d\n", t0.Add(-time.Hour).Unix()))
	cg := "0::/user.slice/session-1.scope\n"
	fakeProc(t, procRoot, 100, 1, "claude", "claude", cg, 3600*100)
	fakeProc(t, procRoot, 200, 1, "claude", "claude", cg, 3600*100)

	proj := filepath.Join(home, ".claude", "projects", "-widgets")
	tx := func(s string) string { return strings.ReplaceAll(s, `"/repo"`, `"`+repo+`"`) }
	write(t, filepath.Join(proj, pubSidA+".jsonl"), tx(userText(0, "add the sprocket"))+
		assistantTool(5, "e1", "Edit", map[string]any{"file_path": filepath.Join(repo, "sprocket.go"), "old_string": "a", "new_string": "b"}))
	write(t, filepath.Join(proj, pubSidB+".jsonl"), tx(userText(10, "tidy the docs"))+assistantEnd(20, "Done."))
	write(t, filepath.Join(home, ".claude", "sessions", "100.json"), fmt.Sprintf(`{"sessionId":%q,"name":"sprocket"}`, pubSidA))
	write(t, filepath.Join(home, ".claude", "sessions", "200.json"), fmt.Sprintf(`{"sessionId":%q}`, pubSidB))

	state := filepath.Join(root, "state")
	write(t, filepath.Join(state, pubSidA+".json"), fmt.Sprintf(
		`{"schema":1,"session_id":%q,"pid":100,"state":"approval","reason":"permission","hook_event":"Notification","observed_at":%q}`, pubSidA, ts(12)))

	cfgPath = filepath.Join(root, "fleet.json")
	write(t, cfgPath, fmt.Sprintf(`{"version":1,"host":"host-a",
 "runtimes":[{"type":"proc","proc_root":%q},{"type":"hookfile","dir":%q,"fresh":"1h"}],
 "agents":[{"type":"claudecode"}],
 "worktrees":{"roots":[{"path":%q,"depth":1}]}}`, procRoot, state, home))
	return root, cfgPath
}

func publicSnapshot(t *testing.T) (Snapshot, string) {
	t.Helper()
	root, cfgPath := publicWorld(t)
	res, err := config.Resolve(fakeEnv(map[string]string{
		"FLEET_CONFIG": cfgPath, "FLEET_HOME": filepath.Join(root, "home", "someone"),
		"FLEET_RESCAN": "1s", "FLEET_PRICES_FILE": testPricesFile(),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return collectSettled(t, collectorFrom(res, nil, nil), t0.Add(30*time.Second)), root
}

func TestPublicEndToEndGolden(t *testing.T) {
	snap, root := publicSnapshot(t)
	by := map[string]SessionView{}
	for _, s := range snap.Sessions {
		by[s.Short] = s
	}
	a, b := by["aaaaaaaa"], by["bbbbbbbb"]
	if a.State != "approval" || a.StateSource != "hook" || a.Repo.Slug() != "example/widgets" || a.Runtime != "proc" {
		t.Errorf("a: %+v", a)
	}
	if b.State != "waiting" || b.StateSource != "transcript" {
		t.Errorf("b: %+v", b)
	}
	out, err := marshalNormalised(snap, root)
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "v1-public-e2e.json", out)
}

// agent_version is the generic key; claude_version repeats it for 0.1 only.
func TestSessionCarriesBothVersionKeys(t *testing.T) {
	snap, _ := publicSnapshot(t)
	raw, err := json.Marshal(snap.Sessions)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 {
		t.Fatalf("rows %d err %v", len(rows), err)
	}
	for _, r := range rows {
		if r["agent_version"] != "2.1.3" || r["claude_version"] != r["agent_version"] {
			t.Errorf("version keys: agent_version=%v claude_version=%v", r["agent_version"], r["claude_version"])
		}
	}
}
