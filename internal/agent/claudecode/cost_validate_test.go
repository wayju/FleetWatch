package claudecode

// Opt-in measurements against real transcripts; skipped unless the env var is
// set, so the normal suite never touches the host's projects tree.
//
//	FLEET_COST_SESSION=<main .jsonl>  prints the session's accumulated cost
//	FLEET_COST_TREE=<projects dir>    runs a full first pass, prints peak RSS

import (
	"encoding/json"
	"fmt"
	"github.com/wayju/fleetwatch/internal/board"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func peakRSSkB() string {
	b, _ := os.ReadFile("/proc/self/status")
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "VmHWM:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "VmHWM:"))
		}
	}
	return "unknown"
}

func waitDone(t *testing.T, a *CostAccumulator, mains []string, limit time.Duration) (board.SessionCost, time.Duration) {
	t.Helper()
	start := time.Now()
	now := time.Unix(1_800_000_000, 0)
	var total board.SessionCost
	for time.Since(start) < limit {
		now = now.Add(2 * costRefresh)
		partial := false
		total = board.EmptyCost(a.prices.AsOf)
		for _, m := range mains {
			c := a.Session(m, now)
			partial = partial || c.Partial
			total.USD += c.USD
			total.Tokens.Add(c.Tokens)
		}
		if !partial {
			return total, time.Since(start)
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("first pass did not finish")
	return total, 0
}

func TestValidateRealSession(t *testing.T) {
	p := os.Getenv("FLEET_COST_SESSION")
	if p == "" {
		t.Skip("FLEET_COST_SESSION not set")
	}
	a := testAcc()
	c := settle(t, a, p, anyCost)
	b, _ := json.MarshalIndent(c, "", " ")
	fmt.Printf("SESSION_COST %s\n", b)
}

func TestMeasureFullFirstPass(t *testing.T) {
	root := os.Getenv("FLEET_COST_TREE")
	if root == "" {
		t.Skip("FLEET_COST_TREE not set")
	}
	var mains []string
	files := 0
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() && strings.HasSuffix(p, ".jsonl") {
			files++
			if !strings.Contains(p, "/subagents/") {
				mains = append(mains, p)
			}
		}
		return nil
	})
	a := testAcc()
	total, took := waitDone(t, a, mains, 20*time.Minute)
	fmt.Printf("FIRST_PASS sessions=%d files=%d duration=%s usd=%.2f peak_rss=%s\n", len(mains), files, took.Round(time.Second), total.USD, peakRSSkB())
}
