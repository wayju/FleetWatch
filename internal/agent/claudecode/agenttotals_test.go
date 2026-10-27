package claudecode

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
)

func TestAgentTotalsFromMetaFiles(t *testing.T) {
	home := t.TempDir()
	c := New(Config{})
	sess := filepath.Join(home, ".claude", "projects", "-home-src-app", "s1.jsonl")
	sub := filepath.Join(strings.TrimSuffix(sess, ".jsonl"), "subagents")
	now := t0.Add(time.Hour)
	meta := func(id string, finished bool, mod time.Time) {
		write(t, filepath.Join(sub, "agent-"+id+".meta.json"), fmt.Sprintf(`{"agentType":"general-purpose","description":"d","finished":%v}`, finished))
		write(t, filepath.Join(sub, "agent-"+id+".jsonl"), "{}\n")
		os.Chtimes(filepath.Join(sub, "agent-"+id+".jsonl"), mod, mod)
	}
	meta("fin1", true, now.Add(-30*time.Minute))
	meta("fin2", true, now.Add(-20*time.Minute))
	meta("run1", false, now.Add(-time.Minute))
	meta("lost", false, now.Add(-40*time.Minute)) // abandoned: not running
	write(t, filepath.Join(sub, "agent-nofin.meta.json"), `{"agentType":"x"}`)

	tot, fin := c.agentTotals(sess, now)
	want := board.AgentTotals{Total: 5, Running: 1, LastFinishedAt: now.Add(-20 * time.Minute).Unix()}
	if tot != want {
		t.Fatalf("got %+v want %+v", tot, want)
	}
	if !fin["fin1"] || !fin["fin2"] || fin["run1"] {
		t.Fatalf("finished set %v", fin)
	}
	// A meta flipping to finished is seen on the next poll.
	meta("run1", true, now)
	if tot, _ = c.agentTotals(sess, now.Add(time.Second)); tot.Running != 0 || tot.LastFinishedAt != now.Unix() {
		t.Fatalf("after finish: %+v", tot)
	}
	if tot, _ = c.agentTotals(filepath.Join(home, "none.jsonl"), now); tot != (board.AgentTotals{}) {
		t.Fatalf("no dir: %+v", tot)
	}
}

// The meta cache holds only what the latest pass saw, so a long-running
// service does not grow with every session it has ever polled.
func TestAgentTotalsCachePrunesUnseen(t *testing.T) {
	home := t.TempDir()
	c := New(Config{})
	sess := filepath.Join(home, ".claude", "projects", "-home-src-app", "s2.jsonl")
	sub := filepath.Join(strings.TrimSuffix(sess, ".jsonl"), "subagents")
	write(t, filepath.Join(sub, "agent-a.meta.json"), `{"finished":true}`)
	c.totals.begin()
	c.agentTotals(sess, t0)
	c.totals.prune() // seen this pass: kept
	if len(c.totals.entries) != 1 {
		t.Fatalf("kept entries %d", len(c.totals.entries))
	}
	c.totals.begin() // next pass never reads the session (ended)
	c.totals.prune()
	if len(c.totals.entries) != 0 {
		t.Fatalf("stale entries %d", len(c.totals.entries))
	}
}
