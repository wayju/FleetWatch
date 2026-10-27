package config

// The forge, worktree and issue sections (see ADR-005): which servers answer for a
// repo's remote host, where repos live on disk, and how a branch name yields an
// issue number. A token is never in the file; token_file and token_env name
// where to read it, and the file itself must live in an encrypted-secrets path.

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/wayju/fleetwatch/internal/forge"
)

// DefaultBranchPattern is the neutral default: a leading number segment such as
// 123-fix or feature/123_fix. One capture group, the issue number.
const DefaultBranchPattern = `(?:^|/)(\d+)[-_]`

const maxRootDepth = 4

// WorktreeRoot is a directory whose repositories are scanned for worktrees.
// Depth 1 means the repos sit directly under Path; 2 means one grouping
// directory between (Path/<group>/<repo>).
type WorktreeRoot struct {
	Path  string
	Depth int
}

type forgeDoc struct {
	Kind      string   `json:"kind"`
	Host      string   `json:"host"`
	Aliases   []string `json:"aliases"`
	APIURL    string   `json:"api_url"`
	WebURL    string   `json:"web_url"`
	TokenFile string   `json:"token_file"`
	TokenEnv  string   `json:"token_env"`
	Owners    []string `json:"owners"`
}

type worktreesSection struct {
	Roots []struct {
		Path  string `json:"path"`
		Depth *int   `json:"depth"`
	} `json:"roots"`
}

type issuesSection struct {
	BranchPattern *string `json:"branch_pattern"`
}

// forgesAndRepos resolves the three sections, then lets the Gitea environment
// variables override the first Gitea entry.
func (r *Resolved) forgesAndRepos(env Env, doc *fileDoc) error {
	if doc == nil {
		doc = &fileDoc{}
	}
	r.Sources["forges"], r.Sources["worktree_roots"], r.Sources["branch_pattern"] = "default", "default", "default"
	r.BranchPattern = DefaultBranchPattern
	r.WorktreeRoots = nil // no roots: the worktree view is off until `worktrees.roots` names some
	for i, raw := range doc.Forges {
		var d forgeDoc
		if err := strict(raw, &d); err != nil {
			return fmt.Errorf("config %s: forges[%d]: %v", r.Path, i, err)
		}
		spec, err := r.forgeSpec(env, d)
		if err != nil {
			return fmt.Errorf("config %s: forges[%d]: %v", r.Path, i, err)
		}
		r.Forges = append(r.Forges, spec)
	}
	if len(doc.Forges) > 0 {
		r.Sources["forges"] = "file"
	}
	if err := r.worktreesFromFile(doc.Worktrees); err != nil {
		return err
	}
	if err := r.issuesFromFile(doc.Issues); err != nil {
		return err
	}
	if err := r.giteaEnv(env); err != nil {
		return err
	}
	return r.checkForgeHosts()
}

func (r *Resolved) forgeSpec(env Env, d forgeDoc) (forge.Spec, error) {
	s := forge.Spec{Kind: d.Kind, Host: strings.ToLower(d.Host), Aliases: lowerAll(d.Aliases),
		APIURL: d.APIURL, WebURL: d.WebURL, Owners: lowerAll(d.Owners)}
	switch d.Kind {
	case "gitea":
		if s.APIURL == "" {
			return s, fmt.Errorf("a gitea forge needs api_url")
		}
	case "github":
		s.Host = defaultStr(s.Host, "github.com")
		s.APIURL = defaultStr(s.APIURL, "https://api.github.com")
		s.WebURL = defaultStr(s.WebURL, "https://github.com")
	default:
		return s, fmt.Errorf("kind must be gitea or github, got %q", d.Kind)
	}
	if err := fillHostAndWeb(&s); err != nil {
		return s, err
	}
	if d.TokenFile != "" && d.TokenEnv != "" {
		return s, fmt.Errorf("set token_file or token_env, not both")
	}
	switch {
	case d.TokenEnv != "":
		s.Token = strings.TrimSpace(env.Get(d.TokenEnv))
	case d.TokenFile != "":
		b, err := env.ReadFile(r.expand(d.TokenFile))
		if err != nil {
			return s, fmt.Errorf("token_file: %v", err)
		}
		s.Token = strings.TrimSpace(string(b))
	}
	return s, nil
}

func defaultStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// checkURL requires an absolute http(s) URL and returns its host name.
func checkURL(field, u string) (string, error) {
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "http" && p.Scheme != "https") || p.Hostname() == "" {
		return "", fmt.Errorf("%s must be an http(s) URL, got %q", field, u)
	}
	return strings.ToLower(p.Hostname()), nil
}

func (r *Resolved) checkForgeHosts() error {
	seen := map[string]bool{}
	for _, f := range r.Forges {
		for _, h := range append([]string{f.Host}, f.Aliases...) {
			if seen[h] {
				return fmt.Errorf("config %s: forge host %q is claimed by two forges", r.Path, h)
			}
			seen[h] = true
		}
	}
	return nil
}

func (r *Resolved) worktreesFromFile(raw json.RawMessage) error {
	if !present(raw) {
		return nil
	}
	var w worktreesSection
	if err := strict(raw, &w); err != nil {
		return fmt.Errorf("config %s: worktrees: %v", r.Path, err)
	}
	if len(w.Roots) == 0 {
		return fmt.Errorf("config %s: worktrees.roots must list at least one root", r.Path)
	}
	r.WorktreeRoots = nil
	for i, root := range w.Roots {
		depth := 1
		if root.Depth != nil {
			depth = *root.Depth
		}
		if root.Path == "" || depth < 1 || depth > maxRootDepth {
			return fmt.Errorf("config %s: worktrees.roots[%d] needs a path and a depth of 1 to %d", r.Path, i, maxRootDepth)
		}
		r.WorktreeRoots = append(r.WorktreeRoots, WorktreeRoot{Path: r.expand(root.Path), Depth: depth})
	}
	r.Sources["worktree_roots"] = "file"
	return nil
}

func (r *Resolved) issuesFromFile(raw json.RawMessage) error {
	if !present(raw) {
		return nil
	}
	var s issuesSection
	if err := strict(raw, &s); err != nil {
		return fmt.Errorf("config %s: issues: %v", r.Path, err)
	}
	if s.BranchPattern == nil {
		return nil
	}
	re, err := regexp.Compile(*s.BranchPattern)
	if err != nil {
		return fmt.Errorf("config %s: issues.branch_pattern: %v", r.Path, err)
	}
	if re.NumSubexp() != 1 {
		return fmt.Errorf("config %s: issues.branch_pattern must have exactly one capture group (the issue number), has %d", r.Path, re.NumSubexp())
	}
	r.BranchPattern, r.Sources["branch_pattern"] = *s.BranchPattern, "file"
	return nil
}

// forgesDescribe is the startup-log line for the forges: kind, host, owner
// scope and whether a token is set, never the token.
func (r *Resolved) forgesDescribe() string {
	var parts []string
	for _, f := range r.Forges {
		owners := "all owners"
		if len(f.Owners) > 0 {
			owners = strings.Join(f.Owners, "+")
		}
		parts = append(parts, fmt.Sprintf("%s@%s(%s, token %v)", f.Kind, f.Host, owners, f.Token != ""))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

func (r *Resolved) rootsDescribe() string {
	var parts []string
	for _, w := range r.WorktreeRoots {
		parts = append(parts, fmt.Sprintf("%s:%d", w.Path, w.Depth))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}
