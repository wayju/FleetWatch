package core

// The collector: one pass joins live processes, hook/HUD state, transcripts,
// subagent transcripts, repositories, accounts and forges into a Snapshot.

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/forge"
	"github.com/wayju/fleetwatch/internal/models"
	"github.com/wayju/fleetwatch/internal/textsafe"
	"strings"
	"time"
)

type Config struct {
	Home           string                // the host user's home, as mounted
	ProcRoot       string                // test profile only: /proc (host pid namespace)
	Tmp            string                // test profile only: host /tmp
	HookDir        string                // test profile only: hook files
	ProjectionPath string                // test profile only: HUD projection.json
	Window         time.Duration         // transcripts touched within this are tailed
	Recent         time.Duration         // an ended session stays on the board this long
	RescanEvery    time.Duration         // directory walk cadence (tails poll every tick)
	ShowText       bool                  // include prompt/reply recap text
	Roots          []config.WorktreeRoot // repo roots scanned for worktrees; none by default (worktree view off)
	BranchPattern  string                // issue number from a branch name; default config.DefaultBranchPattern
	Host           string
	Models         *models.Table // classifies each session's model (see ADR-005)
}

type Collector struct {
	cfg        Config
	repos      *repoCache
	forges     *forge.Lookup
	scan       *issueScan
	recap      *Recapper
	wt         *worktreeIndex
	jobs       []recapJob // sessions offered to the recapper this pass
	rts        runtimeSet
	agents     []agent.Agent
	pricedAsOf string
	servers    map[int][]ServerView // this pass's listeners by Claude pid, from the runtimes
}

// pricer is an agent that prices its sessions; the board's cost carries the
// date of the price table.
type pricer interface{ PricedAsOf() string }

func newCollector(cfg Config, forges *forge.Lookup, recap *Recapper, rts runtimeSet, agents ...agent.Agent) *Collector {
	repos := newRepoCache(30 * time.Second)
	repos.home = cfg.Home
	scan := newIssueScan(cfg.BranchPattern)
	c := &Collector{
		cfg: cfg, rts: rts, agents: agents, repos: repos, forges: forges, scan: scan, recap: recap,
		wt: newWorktreeIndex(cfg.Roots, forges, scan),
	}
	for _, a := range agents {
		if p, ok := a.(pricer); ok && c.pricedAsOf == "" {
			c.pricedAsOf = p.PricedAsOf()
		}
	}
	return c
}

// anonymise swaps the account's email for a short stable hash, so sessions on
// the same login still group together but the address never leaves the host.
func anonymise(a Account, showText bool) Account {
	if a.Email != "" {
		sum := sha256.Sum256([]byte(strings.ToLower(a.Email)))
		a.Email = "acct-" + hex.EncodeToString(sum[:4])
	}
	if !showText {
		a.Name, a.Org = "", ""
		return a
	}
	a.Name, a.Org = textsafe.Clip(textsafe.Redact(a.Name), 60), textsafe.Clip(textsafe.Redact(a.Org), 60)
	return a
}
