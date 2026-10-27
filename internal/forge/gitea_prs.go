package forge

// Gitea's paged pull request list for one repo, newest first, up to a cap.

import (
	"fmt"
	"strings"
	"time"
)

type giteaPR struct {
	Merged bool `json:"merged"`
	Head   struct {
		Ref  string `json:"ref"`
		Repo *struct {
			FullName string `json:"full_name"`
		} `json:"repo"`
	} `json:"head"`
}

// prs returns the repo's list, fetching it when the held copy is stale. The
// lock is held across the fetch so concurrent callers share one round trip.
func (g *gitea) prs(repo string) (prList, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if l, ok := g.lists[repo]; ok && time.Since(l.at) < giteaListTTL {
		return l, nil
	}
	o, name, ok := splitRepo(repo)
	if !ok {
		return prList{}, ErrUnknown
	}
	l := prList{at: time.Now(), merged: map[string]bool{}, seen: map[string]bool{}}
	for page := 1; page <= giteaMaxPages; page++ {
		var items []giteaPR
		path := fmt.Sprintf("/api/v1/repos/%s/%s/pulls?state=all&sort=recentupdate&limit=%d&page=%d", o, name, giteaPageSize, page)
		if err := g.get(path, &items); err != nil {
			return prList{}, err
		}
		for _, it := range items {
			// A fork's branch of the same name is a different branch.
			if it.Head.Repo != nil && !strings.EqualFold(it.Head.Repo.FullName, repo) {
				continue
			}
			l.seen[it.Head.Ref] = true
			if it.Merged {
				l.merged[it.Head.Ref] = true
			}
		}
		if len(items) < giteaPageSize {
			l.complete = true
			break
		}
	}
	g.lists[repo] = l
	return l, nil
}
