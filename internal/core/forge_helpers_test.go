package core

// Test helpers that stand a Gitea forge up behind the production Lookup, so the
// collector tests exercise the real selection path (see ADR-005).

import "github.com/wayju/fleetwatch/internal/forge"

// newGiteaClient is a Gitea forge for the host git.example.test, with no owner limit.
func newGiteaClient(api, token, web string) *forge.Lookup {
	return newGiteaOwned(api, token, web)
}

func newGiteaOwned(api, token, web string, owners ...string) *forge.Lookup {
	if web == "" {
		web = api
	}
	set, err := forge.NewSet([]forge.Spec{{Kind: "gitea", Host: "git.example.test", APIURL: api, WebURL: web, Token: token, Owners: owners}})
	if err != nil {
		panic(err)
	}
	return forge.NewLookup(set)
}

// extractIssues scans with the neutral default branch pattern.
func extractIssues(branch, repoSlug string, texts ...string) []IssueRef {
	return newIssueScan("").extract(branch, repoSlug, texts...)
}
