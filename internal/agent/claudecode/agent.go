package claudecode

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/models"
)

// Config is how the agent finds Claude Code's files.
type Config struct {
	// Dirs are the agent's own config dirs; the first is the default, whose
	// account file sits beside it (<parent>/.claude.json).
	Dirs []host.ConfigDir
	// ExtraDirs adds config dirs a runtime knows about (pool slots)
	// after Dirs. Called on every rescan.
	ExtraDirs   func() []host.ConfigDir
	Window      time.Duration // transcripts touched within this are tailed
	RescanEvery time.Duration // directory walk cadence (tails poll every pass)
	PricesFile  string        // cost.prices / FLEET_PRICES_FILE: the price table; empty prices nothing
	Models      *models.Table // model id -> context window and class; nil knows no model
}

// fileRef is one transcript file in the index.
type fileRef struct {
	path   string
	label  string // the config dir's label ("" for the default)
	cfgDir string // the config dir's path, "" for the default
	mod    time.Time
}

// Agent is the Claude Code agent adapter. It is used from one goroutine, the
// poll loop; the cost accumulator reads in the background.
type Agent struct {
	cfg      Config
	accounts *accountCache
	cost     *CostAccumulator
	totals   totalsCache

	tails    map[string]*tail // main transcript path -> tail
	subTails map[string]*tail // subagent transcript path -> tail
	index    map[string]fileRef
	lastScan time.Time

	now      time.Time                         // this pass
	byID     map[string]*tail                  // this pass's tails by session id
	subs     map[string]map[string]*Transcript // this pass's subagent transcripts by session id
	subsSeen map[string]bool                   // subagent files polled this pass
}

// New builds the agent.
func New(cfg Config) *Agent {
	acc := NewCostAccumulator(loadPrices(cfg.PricesFile))
	acc.models = cfg.Models
	return &Agent{
		cfg: cfg, accounts: &accountCache{}, cost: acc,
		tails: map[string]*tail{}, subTails: map[string]*tail{}, index: map[string]fileRef{},
		byID: map[string]*tail{}, subs: map[string]map[string]*Transcript{}, subsSeen: map[string]bool{},
	}
}

var _ agent.Agent = (*Agent)(nil)

// Name is the agent's config type.
func (*Agent) Name() string { return "claudecode" }

// WaitCostIdle blocks until the background cost reads have finished (tests
// use it to make a snapshot's cost deterministic).
func (a *Agent) WaitCostIdle(timeout time.Duration) bool { return a.cost.WaitIdle(timeout) }

// PricedAsOf is the date of the price table in use.
func (a *Agent) PricedAsOf() string { return a.cost.AsOf() }

// Claims accepts a process named claude, or a node process whose command line
// runs the Claude Code package. The command line is read only for node.
func (*Agent) Claims(comm string, cmdline func() string) bool {
	if comm == "claude" {
		return true
	}
	return strings.HasPrefix(comm, "node") && isClaude(comm, cmdline())
}

func (a *Agent) defaultDir() string {
	if len(a.cfg.Dirs) == 0 {
		return ""
	}
	return a.cfg.Dirs[0].Path
}

// tail starts a pass: it rescans the transcript index on its cadence, then
// polls every transcript touched within the window plus every claimed one.
// Subagent transcripts are polled when a session is asked for (Session).
func (a *Agent) Tail(now time.Time, claimed map[string]bool) {
	if a.lastScan.IsZero() || now.Sub(a.lastScan) >= a.cfg.RescanEvery {
		a.scan()
		a.lastScan = now
	}
	a.totals.begin()
	want := map[string]fileRef{}
	for id, ref := range a.index {
		if now.Sub(ref.mod) <= a.cfg.Window || claimed[id] {
			want[ref.path] = ref
		}
	}
	for path := range a.tails {
		if _, ok := want[path]; !ok {
			delete(a.tails, path)
		}
	}
	for path := range want {
		if a.tails[path] == nil {
			a.tails[path] = newTail(path, strings.TrimSuffix(filepath.Base(path), ".jsonl"))
		}
		_ = a.tails[path].poll(now)
	}
	a.now = now
	a.byID = map[string]*tail{}
	for _, tl := range a.tails {
		a.byID[tl.T.ID] = tl
	}
	a.subs, a.subsSeen = map[string]map[string]*Transcript{}, map[string]bool{}
}

// Session is the folded transcript for id, or nil when none is held.
func (a *Agent) Session(id string) agent.Session {
	tl := a.byID[id]
	if tl == nil {
		return nil
	}
	return &session{a: a, t: tl.T, id: id, ref: a.index[id]}
}

// Recent lists, by id, the sessions with a transcript that are not linked to a
// live process and were active within recent.
func (a *Agent) Recent(now time.Time, recent time.Duration, linked map[string]bool) []string {
	var ids []string
	for id, tl := range a.byID {
		if linked[id] || tl.T.LastActivity.IsZero() || now.Sub(tl.T.LastActivity) > recent {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Account is the identity a config dir is logged in as ("" is the default).
func (a *Agent) Account(configDir string) board.Account {
	if configDir == "" {
		return a.accounts.get(filepath.Join(filepath.Dir(a.defaultDir()), ".claude.json"))
	}
	return a.accounts.get(filepath.Join(configDir, ".claude.json"))
}

// EndPass drops what the pass did not touch: subagent tails, agent totals
// and cost state.
func (a *Agent) EndPass(now time.Time) {
	for f := range a.subTails {
		if !a.subsSeen[f] {
			delete(a.subTails, f)
		}
	}
	a.totals.prune()
	a.cost.Prune(now)
}

// roots lists the transcript directories: the agent's own config dirs, then
// the ones the runtimes know of. A directory reached twice is listed once.
func (a *Agent) roots() []fileRef {
	var out []fileRef
	seen := map[string]bool{}
	add := func(dir, label, cfgDir string) {
		real, err := filepath.EvalSymlinks(dir)
		if err != nil || seen[real] {
			return
		}
		seen[real] = true
		out = append(out, fileRef{path: dir, label: label, cfgDir: cfgDir})
	}
	for i, d := range a.cfg.Dirs {
		cfgDir := d.Path
		if i == 0 {
			cfgDir = ""
		}
		add(filepath.Join(d.Path, "projects"), d.Label, cfgDir)
	}
	if a.cfg.ExtraDirs != nil {
		for _, d := range a.cfg.ExtraDirs() {
			add(filepath.Join(d.Path, "projects"), d.Label, d.Path)
		}
	}
	return out
}

// scan rebuilds the session-id -> file index. It lists names and stats files;
// it reads nothing.
func (a *Agent) scan() {
	idx := map[string]fileRef{}
	for _, root := range a.roots() {
		projects, err := os.ReadDir(root.path)
		if err != nil {
			continue
		}
		for _, p := range projects {
			if p.IsDir() {
				a.indexDir(idx, root, filepath.Join(root.path, p.Name()))
			}
		}
	}
	a.index = idx
}

func (a *Agent) indexDir(idx map[string]fileRef, root fileRef, dir string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		info, err := f.Info() // lstat: a symlink or FIFO is not a transcript
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		id := strings.TrimSuffix(name, ".jsonl")
		if prev, ok := idx[id]; ok && !info.ModTime().After(prev.mod) {
			continue
		}
		idx[id] = fileRef{path: filepath.Join(dir, name), label: root.label, cfgDir: root.cfgDir, mod: info.ModTime()}
	}
}

// subagents polls the subagent transcripts beside a session
// (<id>/subagents/agent-*.jsonl) once per pass and returns them by agent id.
func (a *Agent) subagents(id string) map[string]*Transcript {
	if subs, ok := a.subs[id]; ok {
		return subs
	}
	subs := map[string]*Transcript{}
	a.subs[id] = subs
	tl := a.byID[id]
	if tl == nil {
		return subs
	}
	dir := filepath.Join(strings.TrimSuffix(a.index[id].path, ".jsonl"), "subagents")
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	for _, f := range files {
		st, err := os.Lstat(f)
		if err != nil || !st.Mode().IsRegular() || a.now.Sub(st.ModTime()) > a.cfg.Window {
			continue
		}
		a.subsSeen[f] = true
		sub := a.subTails[f]
		if sub == nil {
			aid := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(f), ".jsonl"), "agent-")
			sub = newTail(f, aid)
			a.subTails[f] = sub
		}
		_ = sub.poll(a.now)
		readSubMeta(f, sub.T)
		subs[sub.T.ID] = sub.T
	}
	return subs
}
