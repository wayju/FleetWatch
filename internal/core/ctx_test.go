package core

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// ctxFixture is one live session (pid 100) with no hook and no projection, so
// the only thing that can place it in a pane is the statusline's ctx file.
func ctxFixture(t *testing.T, tail string) (fixture, Config, string) {
	f, cfg := newFixture(t)
	sid := "eeeeeeee-1111-2222-3333-444444444444"
	proj := filepath.Join(f.home, ".claude", "projects", "-home-src-app")
	write(t, filepath.Join(proj, sid+".jsonl"), userText(0, "hi")+
		assistantTool(5, "t1", "Bash", map[string]any{"command": "ls"})+toolResult(6, "t1", "ok", nil)+
		assistantEnd(9, "done")+tail)
	fakeProc(t, f.proc, 100, 1, "claude", "claude", "0::/user.slice/claude-work-99.scope\n", 3600*100)
	return f, cfg, sid
}

func ctxJSON(pid int, pct int, size any, ts time.Time) string {
	return fmt.Sprintf(`{"pid":%d,"pct":%d,"size":%v,"ts":%d}`, pid, pct, size, ts.Unix())
}
