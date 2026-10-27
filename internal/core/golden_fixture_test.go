package core

// The golden world: one host with a session on every rung of the state ladder
// and every binding path, frozen as testdata/golden/*.json. It is the
// behaviour-preservation proof for the adapter split (see ADR-002): any change to
// the board for these inputs is a diff in a golden file.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	gSidA = "aaaaaaaa-1111-2222-3333-444444444444"
	gSidB = "bbbbbbbb-1111-2222-3333-444444444444"
	gSidC = "cccccccc-1111-2222-3333-444444444444"
	gSidD = "dddddddd-1111-2222-3333-444444444444"
	gSidF = "ffffffff-1111-2222-3333-444444444444"
	gSidG = "99999999-1111-2222-3333-444444444444"
	gSidH = "88888888-1111-2222-3333-444444444444"
	gSidX = "77777777-1111-2222-3333-444444444444" // ended, inside Recent
	gSidY = "66666666-1111-2222-3333-444444444444" // ended, outside Recent
)

type goldenWorld struct {
	tb   *testing.T
	f    fixture
	cfg  Config
	root string // the temp dir holding home, proc and tmp
	proj string // the default config's transcript directory
	repo string
}

func (w goldenWorld) tx(sec int, text string) string {
	return strings.ReplaceAll(userText(sec, text), `"/repo"`, `"`+w.repo+`"`)
}

func (w goldenWorld) transcript(sid, body string) {
	write(w.tb, filepath.Join(w.proj, sid+".jsonl"), body)
}

func hookJSON(state, reason, event string, tsec int64, ppid int, sid string) string {
	return fmt.Sprintf(`{"state":%q,"reason":%q,"event":%q,"ts":%d,"ppid":%d,"session_id":%q}`, state, reason, event, tsec, ppid, sid)
}

func sessionFile(sid, name string) string {
	return fmt.Sprintf(`{"sessionId":%q,"name":%q}`, sid, name)
}

func buildGoldenWorld(t *testing.T, showText bool) goldenWorld {
	t.Helper()
	dir, err := os.MkdirTemp("", "g")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	f, cfg := newFixtureIn(t, dir)
	cfg.ShowText = showText
	w := goldenWorld{tb: t, f: f, cfg: cfg, root: filepath.Dir(f.home), repo: filepath.Join(f.home, "src", "app"),
		proj: filepath.Join(f.home, ".claude", "projects", "-home-src-app")}
	unix0 := t0.Unix()
	const cg = "0::/user.slice/claude-%s-99.scope\n"
	other := "0::/user.slice/session-3.scope\n"
	pool := filepath.Join(f.home, ".claude-alt")
	sess := filepath.Join(f.home, ".claude", "sessions")

	// 100, slot work: projection binding, hook by session id with a dead ppid,
	// a background agent with its own file (delegating), a statusline ctx file,
	// a server, a custom title, mail to B and C, and an Edit in a worktree.
	wtFile := filepath.Join(w.repo, ".claude", "worktrees", "x", "main.go")
	w.transcript(gSidA, w.tx(0, "work on #42")+
		assistantTool(2, "w1", "Edit", map[string]any{"file_path": wtFile, "old_string": "a", "new_string": "b"})+toolResult(3, "w1", "ok", nil)+
		assistantTool(5, "t1", "Agent", map[string]any{"subagent_type": "Explore", "description": "scan", "prompt": "scan the repo", "run_in_background": true})+
		toolResult(6, "t1", "Async agent launched. agentId: sub1", nil)+
		assistantTool(7, "t2", "SendMessage", map[string]any{"to": "bbbbbbbb", "message": "rebase please"})+toolResult(8, "t2", "sent", nil)+
		assistantTool(8, "t4", "SendMessage", map[string]any{"to": "peer-three", "message": "status?"})+toolResult(9, "t4", "sent", nil)+
		assistantEnd(10, "Waiting on you.")+
		line(map[string]any{"type": "custom-title", "customTitle": "Quarterly zebra plan"}))
	write(t, filepath.Join(w.proj, gSidA, "subagents", "agent-sub1.jsonl"),
		line(map[string]any{"type": "user", "isSidechain": true, "agentId": "sub1", "timestamp": ts(5), "message": map[string]any{"role": "user", "content": "scan the repo"}})+
			assistantTool(6, "g1", "Grep", map[string]any{"pattern": "TODO"}))
	fakeProc(t, f.proc, 100, 1, "claude", "claude --model sonnet", fmt.Sprintf(cg, "work"), 3600*100)
	fakeProc(t, f.proc, 101, 100, "node", "node /usr/lib/mcp-server.js", fmt.Sprintf(cg, "work"), 3600*100)
	write(t, filepath.Join(f.hook, "17.json"), hookJSON("finished", "none", "Stop", unix0+10, 5000, gSidA))
	write(t, filepath.Join(f.hook, "ctx", "23.json"), ctxJSON(100, 37, 1000000, t0.Add(10*time.Second)))
	worktree := filepath.Join(w.repo, ".git", "worktrees", "x")
	write(t, filepath.Join(worktree, "gitdir"), filepath.Join(w.repo, ".claude", "worktrees", "x", ".git")+"\n")
	write(t, filepath.Join(worktree, "HEAD"), "ref: refs/heads/fix/55-x\n")
	write(t, filepath.Join(worktree, "index"), "x")

	// 200, slot personal: no projection, no hook; guessed, working on a running tool.
	w.transcript(gSidB, w.tx(10, "tidy")+assistantTool(20, "x1", "Bash", map[string]any{"command": "npm test"}))
	fakeProc(t, f.proc, 200, 1, "claude", "claude", fmt.Sprintf(cg, "personal"), 3600*100)

	// 300, slot s3: session file with a peer name, projection-only state and a
	// ScheduleWakeup, so working with the wake's reason.
	w.transcript(gSidC, w.tx(0, "hello")+
		assistantTool(2, "sw", "ScheduleWakeup", map[string]any{"delaySeconds": 1800, "reason": "check CI"})+
		toolResult(3, "sw", "scheduled", map[string]any{"clampedDelaySeconds": 1800, "wasClamped": false, "scheduledFor": t0.Add(1802 * time.Second).UnixMilli()})+
		assistantEnd(4, "ok"))
	fakeProc(t, f.proc, 300, 1, "claude", "claude", fmt.Sprintf(cg, "s3"), 3600*100)
	write(t, filepath.Join(pool, "s3", "sessions", "300.json"), sessionFile(gSidC, "peer-three"))

	// 400 and 401 resume one session: the newest takes the hook by session id
	// unless the projection's pane disagrees, and then falls back to its ppid.
	w.transcript(gSidD, w.tx(1, "resume work")+assistantEnd(6, "Paused."))
	fakeProc(t, f.proc, 400, 1, "claude", "claude --resume", other, 3600*100)
	fakeProc(t, f.proc, 401, 1, "claude", "claude --resume", other, 3600*100+500)
	write(t, filepath.Join(sess, "400.json"), sessionFile(gSidD, ""))
	write(t, filepath.Join(sess, "401.json"), sessionFile(gSidD, ""))
	write(t, filepath.Join(f.hook, "31.json"), hookJSON("finished", "none", "Stop", unix0+10, 5001, gSidD))
	write(t, filepath.Join(f.hook, "33.json"), hookJSON("working", "none", "PreToolUse", unix0+11, 401, ""))

	// 500, a node launcher with a nested claude 501: no transcript, so starting;
	// the ctx file places its pane.
	fakeProc(t, f.proc, 500, 1, "node", "node /home/u/node_modules/@anthropic-ai/claude-code/cli.js", other, 3000*100)
	fakeProc(t, f.proc, 501, 500, "claude", "claude", other, 3000*100)
	write(t, filepath.Join(f.hook, "ctx", "41.json"), ctxJSON(500, 5, 1000000, t0.Add(-9*time.Minute)))

	// 600, slot work: asking with a decision reason is a question.
	w.transcript(gSidF, w.tx(3, "approve?")+assistantEnd(8, "Which option?"))
	fakeProc(t, f.proc, 600, 1, "claude", "claude", fmt.Sprintf(cg, "work"), 3600*100)
	write(t, filepath.Join(pool, "work", "sessions", "600.json"), sessionFile(gSidF, ""))
	write(t, filepath.Join(f.hook, "51.json"), hookJSON("asking", "decision", "Notification", unix0+12, 5002, gSidF))

	// 700: finished 40 minutes ago, so waiting is demoted to idle; its pending
	// background Bash is capped to zero by the fresh shells scan, so no wake.
	w.transcript(gSidG, w.tx(-3000, "long job")+bgBash(-2990, "b1", "task1")+assistantEnd(-2980, "waiting on build"))
	fakeProc(t, f.proc, 700, 1, "claude", "claude", other, 0)
	write(t, filepath.Join(sess, "700.json"), sessionFile(gSidG, ""))
	write(t, filepath.Join(f.hook, "61.json"), hookJSON("finished", "none", "Stop", unix0-2400, 5003, gSidG))

	// 800: a parked background agent, so a condition wake.
	w.transcript(gSidH, w.tx(-120, "go")+launchParkable(-110)+agentNote(-60, parkedSum("Opus review"))+assistantEnd(-59, "waiting"))
	sub := filepath.Join(w.proj, gSidH, "subagents")
	write(t, filepath.Join(sub, "agent-"+parkedID+".jsonl"), line(map[string]any{"type": "user", "isSidechain": true, "agentId": parkedID, "timestamp": ts(-100),
		"message": map[string]any{"role": "user", "content": "review"}})+assistantEnd(-90, "watching CI"))
	write(t, filepath.Join(sub, "agent-"+parkedID+".meta.json"), `{"agentType":"code-reviewer","description":"Opus review","finished":true}`)
	fakeProc(t, f.proc, 800, 1, "claude", "claude", other, 3600*100)
	write(t, filepath.Join(sess, "800.json"), sessionFile(gSidH, ""))

	// No process: one ended session inside Recent, one outside it.
	w.transcript(gSidX, w.tx(-1300, "ended recently")+assistantEnd(-1200, "done"))
	w.transcript(gSidY, w.tx(-3000, "ended long ago")+assistantEnd(-2700, "done"))

	// One projection file for the sessions it places (one entry per session).
	proj := func(sid, state string, pane string, ppid int, at int) string {
		return fmt.Sprintf(`%q:{"state":%q,"reason":"none","hook_event":"Stop","tmux_pane":%q,"ppid":%d,"observed_at":%q}`, "session:"+sid, state, pane, ppid, ts(at))
	}
	write(t, cfg.ProjectionPath, fmt.Sprintf(`{"fresh_until":%q,"projections":{%s}}`, t0.Add(time.Hour).Format(time.RFC3339),
		strings.Join([]string{
			proj(gSidA, "idle", "%17", 100, 10),
			proj(gSidC, "finished", "%21", 300, 4),
			proj(gSidD, "working", "%40", 401, 8),
			proj(gSidH, "finished", "%60", 800, -58),
		}, ",")))

	// The scanner's file: one server for A, and a fresh empty shells map.
	write(t, filepath.Join(f.hook, serversFile), fmt.Sprintf(`{"scanned_at":%d,"servers":[%s],"shells":{}}`, unix0+30, srvRow(100, 11, 8791, time.Hour)))
	return w
}

// collectSettled runs a pass, waits for the background cost reads it started,
// and runs the pass again so cost is complete and the same on every run.
func collectSettled(t *testing.T, col *Collector, at time.Time) Snapshot {
	t.Helper()
	col.Collect(at)
	for _, a := range col.agents {
		if w, ok := a.(interface{ WaitCostIdle(time.Duration) bool }); ok && !w.WaitCostIdle(10*time.Second) {
			t.Fatal("cost accumulator did not settle")
		}
	}
	return col.Collect(at)
}

func normalizeRoot(s, root string) string {
	return strings.ReplaceAll(s, root, "$ROOT")
}

func appendFile(t *testing.T, path, body string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(body); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, t0.Add(40*time.Second), t0.Add(40*time.Second))
}

// marshalNormalised renders a snapshot the way the goldens are stored.
func marshalNormalised(snap Snapshot, root string) ([]byte, error) {
	out, err := json.MarshalIndent(snap, "", " ")
	if err != nil {
		return nil, err
	}
	return []byte(normalizeRoot(string(out), root) + "\n"), nil
}
