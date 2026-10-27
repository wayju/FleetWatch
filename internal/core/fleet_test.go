package core

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/fleettest"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Fixture builders shared with the adapter packages live in internal/fleettest.
var (
	t0            = fleettest.T0
	ts            = fleettest.TS
	line          = fleettest.Line
	userText      = fleettest.UserText
	assistantTool = fleettest.AssistantTool
	toolResult    = fleettest.ToolResult
	assistantEnd  = fleettest.AssistantEnd
	write         = fleettest.Write
	fakeProc      = fleettest.FakeProc

	wakeCall       = fleettest.WakeCall
	bgBash         = fleettest.BgBash
	monitor        = fleettest.Monitor
	notification   = fleettest.Notification
	agentNote      = fleettest.AgentNote
	parkedSum      = fleettest.ParkedSum
	launchParkable = fleettest.LaunchParkable
	peerLine       = fleettest.PeerLine
	asyncLaunch    = fleettest.AsyncLaunch
	sendMsg        = fleettest.SendMsg
)

const parkedID = fleettest.ParkedID

func applyAll(t *Transcript, s string) {
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		t.Apply([]byte(l))
	}
}

func TestParseRemote(t *testing.T) {
	for in, want := range map[string]string{
		"https://git.example.test/acme/example-stack.git":   "git.example.test acme example-stack",
		"ssh://git@git.example.test:2222/acme/tools.git":    "git.example.test acme tools",
		"git@github.com:wayju/stack-dashboard.git":          "github.com wayju stack-dashboard",
		"https://user:pw@host-a.example.test:3000/org/repo": "host-a.example.test org repo",
		"http://127.0.0.1:1234/git/wayju/example-stack":     "127.0.0.1 wayju example-stack",
	} {
		h, o, n := parseRemote(in)
		if got := h + " " + o + " " + n; got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}

func TestExtractIssues(t *testing.T) {
	got := extractIssues("feature/123-fleet-view", "acme/app",
		"see #45, acme/other#9 and https://git.example.test/acme/app/issues/123")
	want := []string{"acme/app#123 branch", "acme/app#45 mention", "acme/other#9 mention"}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	// URL dedupes against the branch ref; order is branch, url, qualified, bare.
	seen := map[string]bool{}
	for _, r := range got {
		seen[fmt.Sprintf("%s#%d %s", r.Repo, r.Number, r.Source)] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing %s in %+v", w, got)
		}
	}
	if refs := extractIssues("claude/practical-euler-x6rbs7", "a/b"); len(refs) != 0 {
		t.Errorf("session branch is not an issue: %+v", refs)
	}
	if refs := extractIssues("fix/2897-hud", "a/b"); len(refs) != 1 || refs[0].Number != 2897 {
		t.Errorf("fix/ branch: %+v", refs)
	}
}

// --- end to end ---

type fixture struct {
	home, proc, tmp, hook string
}

func newFixture(t *testing.T) (fixture, Config) {
	return newFixtureIn(t, t.TempDir())
}

// newFixtureIn builds the fixture under dir. The golden world passes a short
// path of its own: redact masks the long test-name segment of a t.TempDir path
// inside tool targets, which would hide a worktree edit from attribution.
func newFixtureIn(t *testing.T, dir string) (fixture, Config) {
	f := fixture{home: filepath.Join(dir, "home"), proc: filepath.Join(dir, "proc"), tmp: filepath.Join(dir, "tmp")}
	f.hook = filepath.Join(f.tmp, "hook")
	// Boot at t0-1h, so a process started 3600s+ after boot started at t0.
	write(t, filepath.Join(f.proc, "stat"), fmt.Sprintf("cpu 0\nbtime %d\n", t0.Add(-time.Hour).Unix()))

	// A git checkout the session runs in.
	repo := filepath.Join(f.home, "src", "app")
	write(t, filepath.Join(repo, ".git", "HEAD"), "ref: refs/heads/feature/42-fleet\n")
	write(t, filepath.Join(repo, ".git", "refs", "heads", "feature", "42-fleet"), "0123456789abcdef0123456789abcdef01234567\n")
	write(t, filepath.Join(repo, ".git", "config"), "[core]\n\tbare = false\n[remote \"origin\"]\n\turl = ssh://git@git.example.test:2222/acme/app.git\n")
	write(t, filepath.Join(repo, "VERSION"), "3.4.1\n")

	// Two accounts across two slots.
	write(t, filepath.Join(f.home, ".claude-alt", "work", ".claude.json"), `{"oauthAccount":{"emailAddress":"a@work.example","displayName":"A","organizationName":"Work"},"mcpServers":{"x":{}}}`)
	write(t, filepath.Join(f.home, ".claude-alt", "personal", ".claude.json"), `{"oauthAccount":{"emailAddress":"a@home.example"}}`)
	os.MkdirAll(filepath.Join(f.home, ".claude", "projects"), 0o755)

	cfg := Config{Home: f.home, Roots: []config.WorktreeRoot{{Path: filepath.Join(f.home, "src"), Depth: 1}}, ProcRoot: f.proc, Tmp: f.tmp, HookDir: f.hook,
		ProjectionPath: filepath.Join(f.tmp, "hud", "projection.json"),
		Window:         2 * time.Hour, Recent: 30 * time.Minute, RescanEvery: time.Second, ShowText: true, Host: "host-a"}
	return f, cfg
}

func TestEventsStreamSendsBoardThenTick(t *testing.T) {
	hub := NewHub()
	hub.Publish([]byte(`{"generated_at":1}`), 1, true)
	srv := httptest.NewServer(newMux(hub, &pollHealth{}, time.Second, nil))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/events", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	rd := bufio.NewReader(res.Body)
	next := func() string {
		for {
			l, err := rd.ReadString('\n')
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if strings.HasPrefix(l, "event: ") {
				return strings.TrimSpace(strings.TrimPrefix(l, "event: "))
			}
		}
	}
	if e := next(); e != "board" {
		t.Fatalf("first event %q", e)
	}
	hub.Publish(nil, 2, false)
	if e := next(); e != "tick" {
		t.Fatalf("unchanged board should tick, got %q", e)
	}
	hub.Publish([]byte(`{"generated_at":3}`), 3, true)
	if e := next(); e != "board" {
		t.Fatalf("changed board should resend, got %q", e)
	}
}

func TestHealthIsUnhealthyBeforeFirstPoll(t *testing.T) {
	h := &pollHealth{}
	srv := httptest.NewServer(newMux(NewHub(), h, time.Second, nil))
	defer srv.Close()
	res, _ := http.Get(srv.URL + "/health")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", res.StatusCode)
	}
	h.ok(time.Now(), 3)
	res, _ = http.Get(srv.URL + "/health")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d after a poll", res.StatusCode)
	}
	res, _ = http.Get(srv.URL + "/fleet")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/fleet before a board: %d", res.StatusCode)
	}
}

func TestRecapperSummarisesOutsideTheSession(t *testing.T) {
	var got struct {
		Model    string                     `json:"model"`
		Messages []struct{ Content string } `json:"messages"`
	}
	calls := 0
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		json.NewDecoder(r.Body).Decode(&got)
		fmt.Fprint(w, `{"choices":[{"message":{"content":"<think>hmm</think>Wiring the fleet page; waiting on you to approve the deploy."}}]}`)
	}))
	defer gw.Close()
	r := NewRecapper(gw.URL, "k", "local-qwen", time.Minute)
	if NewRecapper("", "k", "m", time.Minute) != nil || NewRecapper(gw.URL, "", "m", time.Minute) != nil {
		t.Fatal("recapper must be off without a URL and key")
	}

	tr := newTranscript("s1")
	applyAll(tr, userText(0, "build the fleet page, token=abcdefghijkl")+assistantEnd(5, "done"))
	job := recapJob{id: "s1", asOf: tr.LastActivity, digest: digestOf(SessionView{State: "waiting", Branch: "feature/42-x"}, tr)}
	if strings.Contains(job.digest, "abcdefghijkl") {
		t.Fatalf("digest leaks a secret: %s", job.digest)
	}
	r.Offer(t0, []recapJob{job})
	deadline := time.Now().Add(3 * time.Second)
	for {
		if s, _ := r.Get("s1"); s != "" || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if s, at := r.Get("s1"); s != "Wiring the fleet page; waiting on you to approve the deploy." || at == 0 {
		t.Fatalf("summary %q (think tags must go)", s)
	}
	if got.Model != "local-qwen" || !strings.Contains(got.Messages[1].Content, "feature/42-x") {
		t.Fatalf("request %+v", got)
	}
	// No new activity: not summarised again.
	r.Offer(t0.Add(time.Hour), []recapJob{job})
	time.Sleep(100 * time.Millisecond)
	if calls != 1 {
		t.Fatalf("re-summarised an unchanged session: %d calls", calls)
	}
}

// serversFile is the host scanner's file in the hook dir (an optional runtime reads it).
const serversFile = "servers.json"
