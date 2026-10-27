package core

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

const delSID = "dddddddd-1111-2222-3333-444444444444"

// delegatingFixture is a live session whose hook says mainState at t0+9s, with
// optional background agent launched and still running.
func delegatingFixture(t *testing.T, mainState string, runningAgent bool) (fixture, Config) {
	f, cfg := newFixture(t)
	proj := filepath.Join(f.home, ".claude", "projects", "-home-src-app")
	body := userText(0, "hi")
	if runningAgent {
		body += asyncLaunch(1, "t1", "sub1")
	}
	write(t, filepath.Join(proj, delSID+".jsonl"), body+assistantEnd(8, "Waiting on the agent."))
	fakeProc(t, f.proc, 100, 1, "claude", "claude", "0::/user.slice/claude-work-99.scope\n", 3600*100)
	write(t, cfg.ProjectionPath, fmt.Sprintf(`{"fresh_until":%q,"projections":{"session:%s":{"state":%q,"reason":"none","hook_event":"Stop","tmux_pane":"%%17","ppid":100,"observed_at":%q}}}`,
		t0.Add(time.Hour).Format(time.RFC3339), delSID, mainState, ts(9)))
	return f, cfg
}

func agentsJSON(ts int64, sid string) string {
	return fmt.Sprintf(`{"ts":%d,"agent_id":"sub1","session_id":%q}`, ts, sid)
}
