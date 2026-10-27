package core

// Worktrees in flight. A session's own checkout is usually dev or
// master; the work happens in git worktrees that subagents create. Discovery
// reads <repo>/.git/worktrees/*/{gitdir,HEAD} for every repo under the
// configured worktree roots (worktrees.roots; mounted read-only), with the same guards as gitinfo.go: Lstat only,
// capped regular-file reads through readSmall, no symlink following, refs
// validated before use. Writers come from the cwd and file-tool targets in
// each session's and subagent's transcript.

import (
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/forge"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	worktreeRescan   = 60 * time.Second
	worktreeActive   = 10 * time.Minute
	worktreeStale    = 24 * time.Hour
	maxWorktreeRepos = 400
	maxWorktrees     = 2000
	maxWriters       = 10
)

var wtNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// foundWorktree is what discovery reads off disk.
type foundWorktree struct {
	repoDir, repoSlug, repoHost, repoPath string // repoDir is the repo directory's name
	name, path, branch                    string // path is the worktree's checkout, from its gitdir file
	gitAt                                 time.Time
}

type worktreeIndex struct {
	roots  []config.WorktreeRoot
	mu     sync.Mutex
	at     time.Time
	found  []foundWorktree
	hasRun bool
	forges *forge.Lookup // merged state of a branch's PR, from the forge serving its remote
	scan   *issueScan
}

func newWorktreeIndex(roots []config.WorktreeRoot, forges *forge.Lookup, scan *issueScan) *worktreeIndex {
	return &worktreeIndex{roots: roots, forges: forges, scan: scan}
}

// list returns the discovered worktrees, rescanning at most every minute.
func (w *worktreeIndex) list(now time.Time) []foundWorktree {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hasRun || now.Sub(w.at) >= worktreeRescan {
		w.found = discoverWorktrees(w.roots)
		w.at, w.hasRun = now, true
	}
	return w.found
}

// --- view assembly ---

// ownerOf is the index of the longest worktree path containing p, or -1;
// sorted must be ordered by path length, longest first.
func ownerOf(sorted []foundWorktree, p string) int {
	p = filepath.Clean(p)
	for i, f := range sorted {
		if p == f.path || strings.HasPrefix(p, f.path+"/") {
			return i
		}
	}
	return -1
}

type writerKey struct{ session, agent string }

// view joins discovery with the sessions on the board into the snapshot list.
func (w *worktreeIndex) view(now time.Time, sessions []SessionView, srcs map[string]agent.Session) []WorktreeView {
	found := append([]foundWorktree(nil), w.list(now)...)
	sort.SliceStable(found, func(i, j int) bool { return len(found[i].path) > len(found[j].path) })
	writers := make([]map[writerKey]*WorktreeWriter, len(found))
	for _, sv := range sessions {
		src, ok := srcs[sv.ID]
		if !ok {
			continue
		}
		descs := map[string]string{}
		for _, a := range sv.Agents {
			descs[a.ID] = a.Description
		}
		for _, tc := range src.Touches() {
			i := ownerOf(found, tc.Path)
			if i < 0 {
				continue
			}
			if writers[i] == nil {
				writers[i] = map[writerKey]*WorktreeWriter{}
			}
			k := writerKey{sv.ID, tc.Agent}
			wr := writers[i][k]
			if wr == nil {
				wr = &WorktreeWriter{Session: sv.ID, Slot: sv.Slot, AgentID: tc.Agent}
				if tc.Agent != "" {
					wr.AgentDesc = descs[tc.Agent]
				}
				writers[i][k] = wr
			}
			if at := board.Unix(tc.At); at > wr.LastAt {
				wr.LastAt = at
			}
		}
	}
	out := make([]WorktreeView, 0, len(found))
	for i, f := range found {
		out = append(out, w.buildView(now, f, writers[i]))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].PathRel < out[j].PathRel
	})
	return out
}

func (w *worktreeIndex) buildView(now time.Time, f foundWorktree, ws map[writerKey]*WorktreeWriter) WorktreeView {
	repo := f.repoSlug
	if repo == "" {
		repo = f.repoDir
	}
	rel := f.path
	if r, ok := strings.CutPrefix(f.path, f.repoPath+"/"); ok {
		rel = r
	}
	v := WorktreeView{Repo: repo, RepoPath: f.repoPath, Name: f.name, PathRel: rel, Branch: f.branch,
		Writers: []WorktreeWriter{}, LastGitAt: board.Unix(f.gitAt)}
	if n := w.scan.branchIssue(f.branch); n > 0 {
		v.Issue = &n
	}
	newest, recent := fillWriters(&v, ws, now)
	v.Active, v.TwoWriters = recent > 0, recent >= 2
	if last := max(newest, v.LastGitAt); last > 0 {
		v.Stale = now.Sub(time.Unix(last, 0)) > worktreeStale
	}
	if f.branch != "" && f.repoSlug != "" {
		v.Merged = w.forges.Merged(f.repoHost, f.repoSlug, f.branch, now)
	}
	return v
}

// fillWriters orders and caps the writers, returning the newest writer time
// and how many wrote within the active window.
func fillWriters(v *WorktreeView, ws map[writerKey]*WorktreeWriter, now time.Time) (newest int64, recent int) {
	for _, wr := range ws {
		v.Writers = append(v.Writers, *wr)
		newest = max(newest, wr.LastAt)
		if now.Sub(time.Unix(wr.LastAt, 0)) <= worktreeActive {
			recent++
		}
	}
	sort.Slice(v.Writers, func(i, j int) bool {
		a, b := v.Writers[i], v.Writers[j]
		if a.LastAt != b.LastAt {
			return a.LastAt > b.LastAt
		}
		return a.Session+a.AgentID < b.Session+b.AgentID
	})
	if len(v.Writers) > maxWriters {
		v.Writers = v.Writers[:maxWriters]
	}
	return newest, recent
}
