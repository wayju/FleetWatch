package core

// Issue enrichment (see ADR-005): each reference is matched to the forge serving its
// repo's remote host and looked up through the cache, which fetches in the
// background. A reference on a host no forge serves is linked to that host's
// own web UI; one with no known host and no forge admitting its owner is left
// as it is.

import (
	"strconv"
	"time"
)

func (c *Collector) issues(now time.Time, repo RepoInfo, refs []IssueRef) []IssueRef {
	out := []IssueRef{}
	var served []IssueRef
	for _, r := range refs {
		if host := refHost(repo, r); host != "" && c.forges.Match(host, r.Repo) == nil {
			r.URL = "https://" + host + "/" + r.Repo + "/issues/" + strconv.Itoa(r.Number)
			out = append(out, r)
			continue
		}
		served = append(served, r)
	}
	for _, r := range served {
		out = append(out, c.enrich(now, repo, r))
	}
	return out
}

// refHost is the remote host of the reference's repo: known only for the
// session's own repo, since a reference named in a prompt carries no remote.
func refHost(repo RepoInfo, r IssueRef) string {
	if r.Repo == repo.Slug() {
		return repo.Host
	}
	return ""
}

func (c *Collector) enrich(now time.Time, repo RepoInfo, r IssueRef) IssueRef {
	e := c.forges.Match(refHost(repo, r), r.Repo)
	if e == nil || r.Repo == "" {
		return r
	}
	r.URL = e.WebIssueURL(r.Repo, r.Number)
	if is, ok := c.forges.Issue(e, r.Repo, r.Number, now); ok {
		r.Title, r.State, r.IsPR, r.URL = is.Title, is.State, is.IsPR, is.URL
	}
	return r
}
