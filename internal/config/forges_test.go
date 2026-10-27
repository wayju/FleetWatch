package config

import (
	"strings"
	"testing"
)

const baseDoc = `"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]`

func TestForgeDefaults(t *testing.T) {
	r := mustResolve(t, nil, nil)
	if len(r.Forges) != 0 || r.BranchPattern != DefaultBranchPattern {
		t.Fatalf("forges %+v pattern %q", r.Forges, r.BranchPattern)
	}
	if len(r.WorktreeRoots) != 0 {
		t.Fatalf("roots %+v", r.WorktreeRoots)
	}
	for _, k := range []string{"forges", "worktree_roots", "branch_pattern"} {
		if r.Sources[k] != "default" {
			t.Errorf("%s source %q", k, r.Sources[k])
		}
	}
}

func TestForgesWorktreesAndIssuesFromFile(t *testing.T) {
	body := `{"version":1,"home":"/h",` + baseDoc + `,
	 "forges":[
	  {"kind":"gitea","host":"Git.Example","aliases":["git2.example"],"api_url":"https://git.example/","token_env":"G_TOK","owners":["Ops","dev"]},
	  {"kind":"github","token_file":"~/gh.token"}],
	 "worktrees":{"roots":[{"path":"~/src","depth":2},{"path":"/work"}]},
	 "issues":{"branch_pattern":"^(?:feature|fix)/(\\d+)-"}}`
	vars, files := withFile(body)
	vars["G_TOK"] = " tok1 "
	files["/h/gh.token"] = "tok2\n"
	r := mustResolve(t, vars, files)
	g, h := r.Forges[0], r.Forges[1]
	if g.Kind != "gitea" || g.Host != "git.example" || g.APIURL != "https://git.example" || g.WebURL != "https://git.example" ||
		g.Token != "tok1" || strings.Join(g.Owners, ",") != "ops,dev" || g.Aliases[0] != "git2.example" {
		t.Fatalf("gitea %+v", g)
	}
	if h.Host != "github.com" || h.APIURL != "https://api.github.com" || h.WebURL != "https://github.com" || h.Token != "tok2" || len(h.Owners) != 0 {
		t.Fatalf("github %+v", h)
	}
	if r.WorktreeRoots[0] != (WorktreeRoot{"/h/src", 2}) || r.WorktreeRoots[1] != (WorktreeRoot{"/work", 1}) {
		t.Fatalf("roots %+v", r.WorktreeRoots)
	}
	if r.BranchPattern != `^(?:feature|fix)/(\d+)-` || r.Sources["forges"] != "file" || r.Sources["branch_pattern"] != "file" {
		t.Fatalf("pattern %q sources %v", r.BranchPattern, r.Sources)
	}
	for _, l := range r.Describe() {
		if strings.Contains(l, "tok1") || strings.Contains(l, "tok2") {
			t.Fatalf("a token reached the startup log: %s", l)
		}
	}
}

func TestForgeSectionsRejectBadInput(t *testing.T) {
	cases := map[string]struct{ section, want string }{
		"unknown forge key":  {`"forges":[{"kind":"gitea","api_url":"https://g","tok":"x"}]`, "unknown field"},
		"unknown kind":       {`"forges":[{"kind":"gitlab","api_url":"https://g"}]`, "kind must be gitea or github"},
		"gitea needs url":    {`"forges":[{"kind":"gitea","host":"g"}]`, "needs api_url"},
		"bad url":            {`"forges":[{"kind":"gitea","api_url":"git.example"}]`, "http(s) URL"},
		"both token sources": {`"forges":[{"kind":"github","token_env":"A","token_file":"/b"}]`, "not both"},
		"missing token file": {`"forges":[{"kind":"github","token_file":"/nope"}]`, "token_file"},
		"duplicate host":     {`"forges":[{"kind":"github"},{"kind":"github"}]`, "claimed by two forges"},
		"unknown root key":   {`"worktrees":{"roots":[{"path":"/a"}],"depth":2}`, "unknown field"},
		"no roots":           {`"worktrees":{"roots":[]}`, "at least one root"},
		"depth too deep":     {`"worktrees":{"roots":[{"path":"/a","depth":9}]}`, "depth of 1 to 4"},
		"depth zero":         {`"worktrees":{"roots":[{"path":"/a","depth":0}]}`, "depth of 1 to 4"},
		"bad regex":          {`"issues":{"branch_pattern":"(["}`, "branch_pattern"},
		"no capture group":   {`"issues":{"branch_pattern":"\\d+"}`, "exactly one capture group"},
		"two capture groups": {`"issues":{"branch_pattern":"(a)(\\d+)"}`, "exactly one capture group"},
		"unknown issue key":  {`"issues":{"pattern":"(\\d+)"}`, "unknown field"},
	}
	for name, c := range cases {
		vars, files := withFile(`{"version":1,` + baseDoc + `,` + c.section + `}`)
		t.Run(name, func(t *testing.T) { failsWith(t, vars, files, c.want) })
	}
}

func TestGiteaEnvOverridesTheFirstGiteaEntry(t *testing.T) {
	vars, files := withFile(`{"version":1,` + baseDoc + `,"forges":[
	  {"kind":"github"},
	  {"kind":"gitea","api_url":"https://file.example","owners":["a"]}]}`)
	vars["GITEA_URL"], vars["GITEA_WEB_URL"] = "https://env.example", "https://web.example"
	vars["GITEA_TOKEN"], vars["FLEET_GITEA_OWNERS"], vars["FLEET_GITEA_HOSTS"] = "etok", "X, y", "env.example,alias.example"
	r := mustResolve(t, vars, files)
	g := r.Forges[1]
	if g.APIURL != "https://env.example" || g.WebURL != "https://web.example" || g.Token != "etok" ||
		strings.Join(g.Owners, ",") != "x,y" || g.Host != "file.example" || strings.Join(g.Aliases, ",") != "env.example,alias.example" {
		t.Fatalf("%+v", g)
	}
	if r.Forges[0].Kind != "github" || r.Sources["forges"] != "env" {
		t.Fatalf("%+v %v", r.Forges[0], r.Sources)
	}
}

func TestGiteaEnvBuildsAnEntryWithNoFile(t *testing.T) {
	r := mustResolve(t, map[string]string{"GITEA_URL": "https://gitea.example", "GITEA_TOKEN": "t", "FLEET_GITEA_OWNERS": "me"}, nil)
	if len(r.Forges) != 1 {
		t.Fatalf("%+v", r.Forges)
	}
	g := r.Forges[0]
	if g.Kind != "gitea" || g.Host != "gitea.example" || g.WebURL != "https://gitea.example" || g.Token != "t" || g.Owners[0] != "me" {
		t.Fatalf("%+v", g)
	}
}

func TestGiteaEnvWithoutAURLCreatesNothing(t *testing.T) {
	r := mustResolve(t, map[string]string{"GITEA_TOKEN": "t", "FLEET_GITEA_OWNERS": "me"}, nil)
	if len(r.Forges) != 0 {
		t.Fatalf("%+v", r.Forges)
	}
}

func TestEmptyOwnersMeansEveryOwner(t *testing.T) {
	vars, files := withFile(`{"version":1,` + baseDoc + `,"forges":[{"kind":"github"}]}`)
	if r := mustResolve(t, vars, files); len(r.Forges[0].Owners) != 0 {
		t.Fatalf("%+v", r.Forges[0])
	}
}
