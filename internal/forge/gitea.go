package forge

// Gitea: an issue is one GET; a branch's pull request comes from the repo's
// paged PR list, because the list endpoint cannot filter by head. The list is
// held briefly so every branch of one repo shares a single fetch.

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	giteaPageSize = 50
	giteaMaxPages = 4
	giteaListTTL  = 30 * time.Second
)

type gitea struct {
	api, web, token string
	http            *http.Client

	mu    sync.Mutex
	lists map[string]prList
}

type prList struct {
	at       time.Time
	merged   map[string]bool // head branch -> has a merged PR
	seen     map[string]bool // head branch -> has any PR
	complete bool            // every PR was listed, so an absent branch has none
}

func newGitea(s Spec) *gitea {
	web := strings.TrimSuffix(s.WebURL, "/")
	if web == "" {
		web = strings.TrimSuffix(s.APIURL, "/")
	}
	return &gitea{api: strings.TrimSuffix(s.APIURL, "/"), web: web, token: s.Token,
		http: newHTTP(), lists: map[string]prList{}}
}

func (g *gitea) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, g.api+path, nil)
	if err != nil {
		return ErrUnknown
	}
	if g.token != "" {
		req.Header.Set("Authorization", "token "+g.token)
	}
	_, err = doJSON(g.http, req, out)
	return err
}

func (g *gitea) Issue(repo string, n int) (Issue, error) {
	o, name, ok := splitRepo(repo)
	if !ok {
		return Issue{}, ErrUnknown
	}
	var body struct {
		Title       string `json:"title"`
		State       string `json:"state"`
		HTMLURL     string `json:"html_url"`
		PullRequest *struct {
			Merged bool `json:"merged"`
		} `json:"pull_request"`
	}
	if err := g.get(fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d", o, name, n), &body); err != nil {
		return Issue{}, err
	}
	is := Issue{Title: cleanTitle(body.Title), State: body.State, IsPR: body.PullRequest != nil, URL: body.HTMLURL}
	if body.PullRequest != nil && body.PullRequest.Merged {
		is.State = "merged"
	}
	if is.URL == "" {
		is.URL = fmt.Sprintf("%s/%s/issues/%d", g.web, repo, n)
	}
	return is, nil
}

func (g *gitea) PRForBranch(repo, branch string) (*PR, error) {
	if g.token == "" {
		return nil, ErrUnsupported
	}
	l, err := g.prs(repo)
	if err != nil {
		return nil, err
	}
	switch {
	case l.merged[branch]:
		return &PR{Merged: true}, nil
	case l.seen[branch]:
		return &PR{}, nil
	case l.complete:
		return nil, nil
	}
	return nil, ErrUnknown
}
