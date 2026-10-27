package forge

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func mustSet(t *testing.T, specs ...Spec) *Set {
	t.Helper()
	s, err := NewSet(specs)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSetSelectsByHostAndOwner(t *testing.T) {
	s := mustSet(t,
		Spec{Kind: "gitea", Host: "git.example", Aliases: []string{"alias.example"}, APIURL: "http://x", Owners: []string{"ops"}},
		Spec{Kind: "github", Host: "github.com", APIURL: "http://y"})
	cases := []struct {
		host, repo, want string
	}{
		{"git.example", "ops/a", "git.example"},
		{"ALIAS.example", "ops/a", "git.example"},
		{"git.example", "other/a", ""}, // owner not allowed on that forge
		{"github.com", "anyone/a", "github.com"},
		{"nowhere.example", "ops/a", ""},
		{"", "ops/a", "git.example"}, // unknown host: first forge admitting the owner
		{"", "anyone/a", "github.com"},
	}
	for _, c := range cases {
		got := ""
		if e := s.Match(c.host, c.repo); e != nil {
			got = e.Host
		}
		if got != c.want {
			t.Errorf("Match(%q,%q) = %q, want %q", c.host, c.repo, got, c.want)
		}
	}
	if (*Set)(nil).Match("x", "a/b") != nil || !(*Set)(nil).Empty() || s.Empty() || !s.HostKnown("alias.example") || s.HostKnown("zzz") {
		t.Fatal("nil set or host lookup wrong")
	}
}

func TestNewRejectsUnknownKind(t *testing.T) {
	if _, err := New(Spec{Kind: "gitlab"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestGiteaIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token tok" {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/v1/repos/o/r/issues/7":
			w.Write([]byte(`{"title":"A title","state":"open","html_url":"https://web/o/r/issues/7"}`))
		case "/api/v1/repos/o/r/issues/8":
			w.Write([]byte(`{"title":"A PR","state":"closed","pull_request":{"merged":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := newGitea(Spec{APIURL: srv.URL + "/", Token: "tok", WebURL: "https://web"})
	is, err := f.Issue("o/r", 7)
	if err != nil || is.Title != "A title" || is.State != "open" || is.IsPR || is.URL != "https://web/o/r/issues/7" {
		t.Fatalf("%+v %v", is, err)
	}
	is, err = f.Issue("o/r", 8)
	if err != nil || !is.IsPR || is.State != "merged" || is.URL != "https://web/o/r/issues/8" {
		t.Fatalf("%+v %v", is, err)
	}
	if _, err := f.Issue("o/r", 9); err != ErrUnknown {
		t.Fatalf("404 must be unknown, got %v", err)
	}
	if _, err := f.Issue("badrepo", 1); err != ErrUnknown {
		t.Fatalf("got %v", err)
	}
	anon := newGitea(Spec{APIURL: srv.URL})
	if _, err := anon.Issue("o/r", 7); err != ErrUnknown {
		t.Fatalf("a rejected request is unknown, got %v", err)
	}
}

func giteaPRServer(t *testing.T, items string, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token tok" || !strings.HasPrefix(r.URL.Path, "/api/v1/repos/o/r/pulls") {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		hits.Add(1)
		w.Write([]byte(items))
	}))
}

func TestGiteaPRForBranch(t *testing.T) {
	var hits atomic.Int32
	srv := giteaPRServer(t, `[
	 {"merged":true,"head":{"ref":"feat/1-done","repo":{"full_name":"o/r"}}},
	 {"merged":false,"head":{"ref":"feat/2-open","repo":{"full_name":"o/r"}}},
	 {"merged":true,"head":{"ref":"feat/3-fork","repo":{"full_name":"someone/r"}}}]`, &hits)
	defer srv.Close()
	f := newGitea(Spec{APIURL: srv.URL, Token: "tok"})
	for branch, want := range map[string]string{"feat/1-done": "merged", "feat/2-open": "open", "feat/3-fork": "none", "feat/4-none": "none"} {
		pr, err := f.PRForBranch("o/r", branch)
		got := "none"
		if pr != nil {
			got = map[bool]string{true: "merged", false: "open"}[pr.Merged]
		}
		if err != nil || got != want {
			t.Fatalf("%s: want %s, got %s (%v)", branch, want, got, err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("one list call per repo, got %d", hits.Load())
	}
}

func TestGiteaPRNeedsATokenAndAFullListing(t *testing.T) {
	if _, err := newGitea(Spec{APIURL: "http://127.0.0.1:1"}).PRForBranch("o/r", "b"); err != ErrUnsupported {
		t.Fatalf("no token: %v", err)
	}
	var items []string
	for i := 0; i < giteaPageSize; i++ {
		items = append(items, `{"merged":false,"head":{"ref":"b`+strconv.Itoa(i)+`","repo":{"full_name":"o/r"}}}`)
	}
	var hits atomic.Int32
	srv := giteaPRServer(t, "["+strings.Join(items, ",")+"]", &hits)
	defer srv.Close()
	f := newGitea(Spec{APIURL: srv.URL, Token: "tok"})
	if _, err := f.PRForBranch("o/r", "never-listed"); err != ErrUnknown {
		t.Fatalf("a branch absent from a truncated listing is unknown, got %v", err)
	}
	if hits.Load() != giteaMaxPages {
		t.Fatalf("pages capped at %d, got %d", giteaMaxPages, hits.Load())
	}
	if pr, err := f.PRForBranch("o/r", "b3"); err != nil || pr == nil || pr.Merged {
		t.Fatalf("listed unmerged branch: %v %v", pr, err)
	}
	bad := newGitea(Spec{APIURL: srv.URL, Token: "wrong"})
	if _, err := bad.PRForBranch("o/r", "b3"); err != ErrUnknown {
		t.Fatalf("a failed lookup is unknown, got %v", err)
	}
}

func TestLookupCachesAndFetchesInTheBackground(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"title":"T","state":"open"}`))
	}))
	defer srv.Close()
	set := mustSet(t, Spec{Kind: "gitea", Host: "g", APIURL: srv.URL, WebURL: "https://web"})
	l := NewLookup(set)
	e := l.Match("g", "o/r")
	if _, ok := l.Issue(e, "o/r", 1, t0); ok {
		t.Fatal("the first call must not wait for the forge")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if is, ok := l.Issue(e, "o/r", 1, t0); ok {
			if is.Title != "T" {
				t.Fatalf("%+v", is)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("answer never landed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	l.Issue(e, "o/r", 1, t0.Add(time.Minute))
	if hits.Load() != 1 {
		t.Fatalf("a fresh entry must not refetch, got %d calls", hits.Load())
	}
	if (*Lookup)(nil).Match("g", "o/r") != nil || l.Merged("", "o/r", "b", t0) != nil || l.Merged("other", "o/r", "b", t0) != nil {
		t.Fatal("nil lookup, unknown host or empty host must answer nothing")
	}
}
