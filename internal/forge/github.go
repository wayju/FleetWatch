package forge

// GitHub over REST v3. The token is optional: public repos answer without one,
// at GitHub's lower anonymous rate limit, which the limiter respects.

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type github struct {
	api, web, token string
	http            *http.Client
	lim             *limiter
}

func newGitHub(s Spec) *github {
	api := strings.TrimSuffix(s.APIURL, "/")
	if api == "" {
		api = "https://api.github.com"
	}
	web := strings.TrimSuffix(s.WebURL, "/")
	if web == "" {
		web = "https://github.com"
	}
	return &github{api: api, web: web, token: s.Token, http: newHTTP(), lim: &limiter{now: time.Now}}
}

func (g *github) get(path string, out any) error {
	if err := g.lim.wait(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, g.api+path, nil)
	if err != nil {
		return ErrUnknown
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fleetwatch")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}
	res, err := doJSON(g.http, req, out)
	g.lim.observe(res)
	return err
}

func (g *github) Issue(repo string, n int) (Issue, error) {
	o, name, ok := splitRepo(repo)
	if !ok {
		return Issue{}, ErrUnknown
	}
	var body struct {
		Title       string `json:"title"`
		State       string `json:"state"`
		HTMLURL     string `json:"html_url"`
		PullRequest *struct {
			MergedAt *string `json:"merged_at"`
		} `json:"pull_request"`
	}
	if err := g.get(fmt.Sprintf("/repos/%s/%s/issues/%d", o, name, n), &body); err != nil {
		return Issue{}, err
	}
	is := Issue{Title: cleanTitle(body.Title), State: body.State, IsPR: body.PullRequest != nil, URL: body.HTMLURL}
	if body.PullRequest != nil && body.PullRequest.MergedAt != nil {
		is.State = "merged"
	}
	if is.URL == "" {
		is.URL = fmt.Sprintf("%s/%s/issues/%d", g.web, repo, n)
	}
	return is, nil
}

// PRForBranch asks GitHub for the pulls whose head is owner:branch, so one
// small request answers it however many PRs the repo has.
func (g *github) PRForBranch(repo, branch string) (*PR, error) {
	o, name, ok := splitRepo(repo)
	if !ok {
		return nil, ErrUnknown
	}
	owner, _, _ := strings.Cut(repo, "/")
	var items []struct {
		MergedAt *string `json:"merged_at"`
		Head     struct {
			Ref  string `json:"ref"`
			Repo *struct {
				FullName string `json:"full_name"`
			} `json:"repo"`
		} `json:"head"`
	}
	q := url.Values{"state": {"all"}, "head": {owner + ":" + branch}, "per_page": {"30"}}
	if err := g.get(fmt.Sprintf("/repos/%s/%s/pulls?%s", o, name, q.Encode()), &items); err != nil {
		return nil, err
	}
	var found *PR
	for _, it := range items {
		if it.Head.Ref != branch || (it.Head.Repo != nil && !strings.EqualFold(it.Head.Repo.FullName, repo)) {
			continue
		}
		if it.MergedAt != nil {
			return &PR{Merged: true}, nil
		}
		found = &PR{}
	}
	return found, nil
}
