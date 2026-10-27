package core

// Repository facts for a session's working directory, read straight from the
// .git directory (no git binary in the image): the origin remote, the checked
// out branch and commit, and the project's own version where it declares one.
// Worktrees are followed through their `gitdir:` file and `commondir`.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Every read below goes through readSmall: capped, regular files only, no
// symlinks, never blocking on a FIFO. A transcript's cwd is untrusted input, so
// a checkout is only believed when its git directory resolves under home.

const (
	maxGitFile  = 1 << 20  // config, HEAD, a ref, a pointer file
	maxPackRefs = 16 << 20 // packed-refs
)

var (
	headRefRe = regexp.MustCompile(`^refs/heads/[A-Za-z0-9._/-]+$`)
	hashRe    = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
)

// validRef is a branch ref safe to join onto the git directory.
func validRef(ref string) bool {
	return headRefRe.MatchString(ref) && !strings.Contains(ref, "..")
}

// under reports whether p, with symlinks resolved, lies inside base.
func under(base, p string) bool {
	rb, err := filepath.EvalSymlinks(base)
	if err != nil {
		return false
	}
	rp, err := filepath.EvalSymlinks(p)
	if err != nil {
		return false
	}
	return rp == rb || strings.HasPrefix(rp, strings.TrimSuffix(rb, "/")+"/")
}

func findGitDir(home, cwd string) (root, gitDir string, worktree bool) {
	dir := filepath.Clean(cwd)
	for i := 0; i < 40 && dir != "" && dir != "/"; i++ {
		p := filepath.Join(dir, ".git")
		st, err := os.Lstat(p)
		if err == nil {
			if st.IsDir() {
				if under(home, p) {
					return dir, p, false
				}
				return "", "", false
			}
			b, err := fsafe.ReadSmall(p, maxGitFile)
			if err == nil && strings.HasPrefix(string(b), "gitdir:") {
				gd := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
				if !filepath.IsAbs(gd) {
					gd = filepath.Join(dir, gd)
				}
				if under(home, gd) {
					return dir, filepath.Clean(gd), true
				}
				return "", "", false
			}
		}
		dir = filepath.Dir(dir)
	}
	return "", "", false
}

func commonDir(home, gitDir string) string {
	b, err := fsafe.ReadSmall(filepath.Join(gitDir, "commondir"), maxGitFile)
	if err != nil {
		return gitDir
	}
	cd := strings.TrimSpace(string(b))
	if !filepath.IsAbs(cd) {
		cd = filepath.Join(gitDir, cd)
	}
	cd = filepath.Clean(cd)
	if !under(home, cd) {
		return gitDir
	}
	return cd
}

func resolveRef(common, ref string) string {
	if !validRef(ref) {
		return ""
	}
	p := filepath.Join(common, ref)
	if under(common, p) {
		if b, err := fsafe.ReadSmall(p, 4096); err == nil {
			if h := strings.TrimSpace(string(b)); hashRe.MatchString(h) {
				return h
			}
			return ""
		}
	}
	b, err := fsafe.ReadSmall(filepath.Join(common, "packed-refs"), maxPackRefs)
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		fields := strings.Fields(ln)
		if len(fields) == 2 && fields[1] == ref && hashRe.MatchString(fields[0]) {
			return fields[0]
		}
	}
	return ""
}

func originURL(common string) string {
	b, err := fsafe.ReadSmall(filepath.Join(common, "config"), maxGitFile)
	if err != nil {
		return ""
	}
	inOrigin := false
	for _, ln := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(ln)
		if strings.HasPrefix(line, "[") {
			inOrigin = line == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(line, "url") {
			if i := strings.IndexByte(line, '='); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

var scpRemoteRe = regexp.MustCompile(`^(?:[^@/]+@)?([^:/]+):(.+)$`)

// parseRemote takes https://, ssh:// and scp-style (git@host:owner/repo)
// remotes to host, owner and repo name.
func parseRemote(u string) (host, owner, name string) {
	u = strings.TrimSuffix(strings.TrimSpace(u), "/")
	u = strings.TrimSuffix(u, ".git")
	var path string
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if at := strings.LastIndexByte(strings.SplitN(rest, "/", 2)[0], '@'); at >= 0 {
			rest = rest[at+1:]
		}
		parts := strings.SplitN(rest, "/", 2)
		host = parts[0]
		if j := strings.IndexByte(host, ':'); j >= 0 {
			host = host[:j]
		}
		if len(parts) == 2 {
			path = parts[1]
		}
	} else if m := scpRemoteRe.FindStringSubmatch(u); m != nil {
		host, path = m[1], m[2]
	} else {
		return "", "", ""
	}
	segs := strings.Split(strings.Trim(path, "/"), "/")
	if len(segs) < 2 {
		return host, "", ""
	}
	return host, segs[len(segs)-2], segs[len(segs)-1]
}

var pyprojectVersionRe = regexp.MustCompile(`(?m)^version\s*=\s*["']([^"']+)["']`)

// projectVersion is the version the project declares about itself: a VERSION
// file wins, then package.json, then pyproject.toml.
func projectVersion(root string) (string, string) {
	if b, err := fsafe.ReadSmall(filepath.Join(root, "VERSION"), maxGitFile); err == nil {
		if v := strings.TrimSpace(string(b)); v != "" && len(v) < 40 {
			return v, "VERSION"
		}
	}
	if b, err := fsafe.ReadSmall(filepath.Join(root, "package.json"), maxGitFile); err == nil {
		var p struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(b, &p) == nil && p.Version != "" {
			return p.Version, "package.json"
		}
	}
	if b, err := fsafe.ReadSmall(filepath.Join(root, "pyproject.toml"), maxGitFile); err == nil {
		if m := pyprojectVersionRe.FindSubmatch(b); m != nil {
			return string(m[1]), "pyproject.toml"
		}
	}
	return "", ""
}

func readRepo(home, cwd string) RepoInfo {
	root, gitDir, wt := findGitDir(home, cwd)
	if root == "" {
		return RepoInfo{}
	}
	common := commonDir(home, gitDir)
	r := RepoInfo{Root: root, Worktree: wt}
	r.Remote = originURL(common)
	r.Host, r.Owner, r.Name = parseRemote(r.Remote)
	if r.Name == "" {
		r.Name = filepath.Base(root)
	}
	if b, err := fsafe.ReadSmall(filepath.Join(gitDir, "HEAD"), 4096); err == nil {
		head := strings.TrimSpace(string(b))
		if ref, ok := strings.CutPrefix(head, "ref: "); ok {
			if validRef(ref) {
				r.Branch = strings.TrimPrefix(ref, "refs/heads/")
				r.Commit = resolveRef(common, ref)
			}
		} else if hashRe.MatchString(head) {
			r.Commit = head
		}
	}
	if len(r.Commit) > 9 {
		r.Commit = r.Commit[:9]
	}
	r.Version, r.VersionSrc = projectVersion(root)
	return r
}

// repoCache keeps readRepo off the hot path: a cwd is re-read at most every ttl.
type repoCache struct {
	home string // checkouts outside it are not read
	mu   sync.Mutex
	ttl  time.Duration
	m    map[string]repoEntry
}

type repoEntry struct {
	at   time.Time
	info RepoInfo
}

func newRepoCache(ttl time.Duration) *repoCache {
	return &repoCache{ttl: ttl, m: map[string]repoEntry{}}
}

func (c *repoCache) get(cwd string, now time.Time) RepoInfo {
	if cwd == "" {
		return RepoInfo{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[cwd]; ok && now.Sub(e.at) < c.ttl {
		return e.info
	}
	info := readRepo(c.home, cwd)
	c.m[cwd] = repoEntry{at: now, info: info}
	return info
}
