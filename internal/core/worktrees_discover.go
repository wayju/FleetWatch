package core

// Worktree discovery: which repos under the configured roots have
// git worktrees, read straight off disk with the same guards as gitinfo.go
// (Lstat only, capped regular-file reads, no symlink following, validated refs).

import (
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func isRealDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.IsDir()
}

// discoverWorktrees walks each root to its configured depth. A repo (a real
// .git directory) ends the descent; the directories above it only group repos.
func discoverWorktrees(roots []config.WorktreeRoot) []foundWorktree {
	var out []foundWorktree
	repos := 0
	paths := rootPaths(roots)
	for _, root := range roots {
		for _, repoPath := range findRepos(root.Path, root.Depth) {
			if repos >= maxWorktreeRepos || len(out) >= maxWorktrees {
				return out
			}
			gitDir := filepath.Join(repoPath, ".git")
			if !isRealDir(filepath.Join(gitDir, "worktrees")) {
				continue
			}
			repos++
			out = append(out, repoWorktrees(paths, filepath.Base(repoPath), repoPath, gitDir)...)
		}
	}
	return out
}

func rootPaths(roots []config.WorktreeRoot) []string {
	ps := make([]string, len(roots))
	for i, r := range roots {
		ps[i] = r.Path
	}
	return ps
}

// findRepos lists the directories holding a real .git directory, searching
// depth levels below dir. DirEntry types come from lstat, so a symlink is never
// followed; below the first level, dot-directories are not searched.
func findRepos(dir string, depth int) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join(dir, e.Name())
		switch {
		case isRealDir(filepath.Join(p, ".git")):
			out = append(out, p)
		case depth > 1 && !strings.HasPrefix(e.Name(), "."):
			out = append(out, findRepos(p, depth-1)...)
		}
	}
	return out
}

func repoWorktrees(roots []string, repoDir, repoPath, gitDir string) []foundWorktree {
	ents, err := os.ReadDir(filepath.Join(gitDir, "worktrees"))
	if err != nil {
		return nil
	}
	host, owner, name := parseRemote(originURL(gitDir))
	slug := ""
	if owner != "" && name != "" {
		slug = owner + "/" + name
	}
	var out []foundWorktree
	for _, e := range ents {
		if !e.IsDir() || !wtNameRe.MatchString(e.Name()) {
			continue
		}
		admin := filepath.Join(gitDir, "worktrees", e.Name())
		path := worktreePath(roots, admin)
		if path == "" {
			continue
		}
		out = append(out, foundWorktree{
			repoDir: repoDir, repoSlug: slug, repoHost: host, repoPath: repoPath, name: e.Name(),
			path: path, branch: worktreeBranch(admin), gitAt: lastGitActivity(admin),
		})
	}
	return out
}

// worktreePath is the checkout named by the admin dir's gitdir file ("<path>/.git"),
// or "" when it is not an absolute path under one of the roots.
func worktreePath(roots []string, admin string) string {
	b, err := fsafe.ReadSmall(filepath.Join(admin, "gitdir"), maxGitFile)
	if err != nil {
		return ""
	}
	gd := filepath.Clean(strings.TrimSpace(string(b)))
	if !filepath.IsAbs(gd) || filepath.Base(gd) != ".git" {
		return ""
	}
	p := filepath.Dir(gd)
	if !underAny(p, roots) {
		return ""
	}
	return p
}

// worktreeBranch is the checked-out branch, or "" for a detached or invalid HEAD.
func worktreeBranch(admin string) string {
	b, err := fsafe.ReadSmall(filepath.Join(admin, "HEAD"), 4096)
	if err != nil {
		return ""
	}
	if ref, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "ref: "); ok && validRef(ref) {
		return strings.TrimPrefix(ref, "refs/heads/")
	}
	return ""
}

// lastGitActivity is the newest mtime of index, HEAD and logs/HEAD.
func lastGitActivity(admin string) time.Time {
	var newest time.Time
	for _, f := range []string{"index", "HEAD", filepath.Join("logs", "HEAD")} {
		if st, err := os.Lstat(filepath.Join(admin, f)); err == nil && st.Mode().IsRegular() && st.ModTime().After(newest) {
			newest = st.ModTime()
		}
	}
	return newest
}
