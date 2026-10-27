package core

// Agent-started dev servers. Agents start `python3 -m http.server`, vite or a
// scratch serve script to preview their work and leave it running. A runtime
// that can see them reports them on its Observation (an optional pool runtime
// reads a servers.json a host-side scanner writes; the file format, its
// validation and its freshness window live in that package). This file flags
// stale servers and attaches them to sessions by agent pid.

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	serverStale   = 12 * time.Hour
	staleAge      = "age"
	staleMerged   = "merged"
	staleDirGone  = "dir_gone"
	worktreeInDir = "/.claude/worktrees/"
)

// markServersStale flags servers older than 12 h, serving a branch that has
// merged, or serving a worktree directory that no longer exists. The
// worktree roots bound the existence check to paths the container can see.
func markServersStale(servers []ServerView, now time.Time, wts []WorktreeView, roots []string) {
	for i := range servers {
		s := &servers[i]
		switch {
		case now.Sub(time.Unix(s.StartedAt, 0)) > serverStale:
			s.Stale, s.StaleReason = true, staleAge
		case s.Dir != "" && inMergedWorktree(s.Dir, wts):
			s.Stale, s.StaleReason = true, staleMerged
		case s.Dir != "" && strings.Contains(s.Dir, worktreeInDir) && underAny(s.Dir, roots) && missing(s.Dir):
			s.Stale, s.StaleReason = true, staleDirGone
		}
	}
}

func inMergedWorktree(dir string, wts []WorktreeView) bool {
	for _, w := range wts {
		if w.Merged == nil || !*w.Merged || w.RepoPath == "" {
			continue
		}
		p := filepath.Join(w.RepoPath, w.PathRel)
		if dir == p || strings.HasPrefix(dir, p+"/") {
			return true
		}
	}
	return false
}

func underAny(p string, roots []string) bool {
	for _, root := range roots {
		if root != "" && strings.HasPrefix(p, strings.TrimSuffix(filepath.Clean(root), "/")+"/") {
			return true
		}
	}
	return false
}

func missing(p string) bool {
	_, err := os.Lstat(p)
	return os.IsNotExist(err)
}

// attachServers fills every live session's Servers and the fleet counts. It runs
// after the worktree view is built (staleness reads Merged) and before RepoPath
// is blanked.
func (c *Collector) attachServers(snap *Snapshot, now time.Time) {
	byPID := c.servers
	for i := range snap.Sessions {
		v := &snap.Sessions[i]
		v.Servers = []ServerView{}
		if !v.Live || v.PID == 0 {
			continue
		}
		v.Servers = append(v.Servers, byPID[v.PID]...)
		markServersStale(v.Servers, now, snap.Worktrees, rootPaths(c.cfg.Roots))
		for j := range v.Servers {
			snap.Counts.Servers++
			if v.Servers[j].Stale {
				snap.Counts.ServersStale++
			}
			if !c.cfg.ShowText {
				v.Servers[j].Cmd, v.Servers[j].Cwd, v.Servers[j].Dir = "", "", ""
			}
		}
	}
}
