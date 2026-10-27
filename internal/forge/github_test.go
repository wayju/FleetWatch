package forge

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestGitHubIssueWithoutAToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("User-Agent") == "" || r.Header.Get("Accept") != "application/vnd.github+json" {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		switch r.URL.Path {
		case "/repos/o/r/issues/1":
			w.Write([]byte(`{"title":"Public issue","state":"open","html_url":"https://github.com/o/r/issues/1"}`))
		case "/repos/o/r/issues/2":
			w.Write([]byte(`{"title":"Merged PR","state":"closed","pull_request":{"merged_at":"2026-10-01T00:00:00Z"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	f := newGitHub(Spec{APIURL: srv.URL})
	if is, err := f.Issue("o/r", 1); err != nil || is.Title != "Public issue" || is.State != "open" || is.IsPR {
		t.Fatalf("%+v %v", is, err)
	}
	is, err := f.Issue("o/r", 2)
	if err != nil || !is.IsPR || is.State != "merged" || is.URL != "https://github.com/o/r/issues/2" {
		t.Fatalf("%+v %v", is, err)
	}
	if _, err := f.Issue("o/r", 3); err != ErrUnknown {
		t.Fatalf("got %v", err)
	}
}

func TestGitHubSendsATokenWhenGiven(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"title":"Private","state":"open"}`))
	}))
	defer srv.Close()
	if is, err := newGitHub(Spec{APIURL: srv.URL, Token: "secret"}).Issue("o/r", 1); err != nil || is.Title != "Private" {
		t.Fatalf("%+v %v", is, err)
	}
}

func TestGitHubPRForBranch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/pulls" || r.URL.Query().Get("state") != "all" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Query().Get("head") {
		case "o:feat/1-done":
			w.Write([]byte(`[{"merged_at":null,"head":{"ref":"feat/1-done","repo":{"full_name":"o/r"}}},
			 {"merged_at":"2026-10-01T00:00:00Z","head":{"ref":"feat/1-done","repo":{"full_name":"o/r"}}}]`))
		case "o:feat/2-open":
			w.Write([]byte(`[{"merged_at":null,"head":{"ref":"feat/2-open","repo":{"full_name":"o/r"}}}]`))
		case "o:feat/3-fork":
			w.Write([]byte(`[{"merged_at":"2026-10-01T00:00:00Z","head":{"ref":"feat/3-fork","repo":{"full_name":"fork/r"}}}]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()
	f := newGitHub(Spec{APIURL: srv.URL})
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
}

func TestGitHubBacksOffUntilTheRateLimitResets(t *testing.T) {
	var hits atomic.Int32
	now := t0
	reset := t0.Add(10 * time.Minute)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset.Unix(), 10))
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	defer srv.Close()
	f := newGitHub(Spec{APIURL: srv.URL})
	f.lim.now = func() time.Time { return now }
	if _, err := f.Issue("o/r", 1); err != ErrUnknown {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		f.Issue("o/r", 1)
	}
	if hits.Load() != 1 {
		t.Fatalf("calls during the backoff must not reach the server, got %d", hits.Load())
	}
	now = reset.Add(time.Second)
	f.Issue("o/r", 1)
	if hits.Load() != 2 {
		t.Fatalf("after the reset the next call goes out, got %d", hits.Load())
	}
}

func TestRetryAfterAndExhaustedBudgetOnSuccess(t *testing.T) {
	now := t0
	l := &limiter{now: func() time.Time { return now }}
	h := http.Header{}
	h.Set("Retry-After", "30")
	l.observe(&http.Response{StatusCode: http.StatusTooManyRequests, Header: h})
	if l.wait() == nil {
		t.Fatal("Retry-After must start a backoff")
	}
	now = t0.Add(31 * time.Second)
	if l.wait() != nil {
		t.Fatal("backoff must end")
	}
	h = http.Header{}
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", strconv.FormatInt(now.Add(24*time.Hour).Unix(), 10))
	l.observe(&http.Response{StatusCode: http.StatusOK, Header: h})
	now = now.Add(maxBackoff + time.Second)
	if l.wait() != nil {
		t.Fatal("a far-off reset is capped at the maximum backoff")
	}
	h = http.Header{}
	h.Set("X-RateLimit-Remaining", "12")
	l.observe(&http.Response{StatusCode: http.StatusOK, Header: h})
	if l.wait() != nil {
		t.Fatal("budget left means no backoff")
	}
}
