package core

// Which issues a session is working on. The branch name is the strongest
// signal (issues.branch_pattern, one capture group for the number); issue URLs
// and #refs in the session's prompts come next. Titles and state come from the
// forge that serves the repo's remote host (issues_enrich.go; see ADR-005).

import (
	"github.com/wayju/fleetwatch/internal/config"
	"regexp"
	"strconv"
)

var (
	issueURLRe  = regexp.MustCompile(`https?://[^\s/]+/([^\s/]+)/([^\s/]+)/(issues|pulls)/(\d{1,6})`)
	qualifiedRe = regexp.MustCompile(`\b([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+)#(\d{1,6})\b`)
	mentionRe   = regexp.MustCompile(`(?:^|[\s(\[])#(\d{1,6})\b`)
)

const (
	maxIssues = 5
	maxIssueN = 999999
)

// issueScan finds issue references; it holds the configured branch pattern.
type issueScan struct{ branchRe *regexp.Regexp }

// newIssueScan compiles a pattern the config has already validated (one
// capture group); an empty pattern is the neutral default.
func newIssueScan(pattern string) *issueScan {
	if pattern == "" {
		pattern = config.DefaultBranchPattern
	}
	return &issueScan{branchRe: regexp.MustCompile(pattern)}
}

// branchIssue is the issue number a branch name carries, or 0.
func (s *issueScan) branchIssue(branch string) int {
	m := s.branchRe.FindStringSubmatch(branch)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n > maxIssueN {
		return 0
	}
	return n
}

func (s *issueScan) extract(branch, repoSlug string, texts ...string) []IssueRef {
	var out []IssueRef
	seen := map[string]bool{}
	add := func(repo string, n int, src string) {
		if n <= 0 || len(out) >= maxIssues {
			return
		}
		k := repo + "#" + strconv.Itoa(n)
		if seen[k] {
			return
		}
		seen[k] = true
		out = append(out, IssueRef{Repo: repo, Number: n, Source: src})
	}
	add(repoSlug, s.branchIssue(branch), "branch")
	for _, txt := range texts {
		for _, m := range issueURLRe.FindAllStringSubmatch(txt, -1) {
			n, _ := strconv.Atoi(m[4])
			add(m[1]+"/"+m[2], n, "url")
		}
		for _, m := range qualifiedRe.FindAllStringSubmatch(txt, -1) {
			n, _ := strconv.Atoi(m[2])
			add(m[1], n, "mention")
		}
		for _, m := range mentionRe.FindAllStringSubmatch(txt, -1) {
			n, _ := strconv.Atoi(m[1])
			add(repoSlug, n, "mention")
		}
	}
	return out
}
