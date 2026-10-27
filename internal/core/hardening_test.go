package core

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestHostGuard(t *testing.T) {
	h := hostGuard([]string{"host-a.example.test:8909", "localhost:8909"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	for host, want := range map[string]int{
		"host-a.example.test:8909": 200, "LOCALHOST:8909": 200,
		"evil.example:8909": 421, "host-a.example.test": 421, "127.0.0.1:8909": 421, "": 421,
	} {
		req := httptest.NewRequest("GET", "/fleet", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("host %q: %d, want %d", host, rec.Code, want)
		}
	}
}

func TestEventsSubscriberCap(t *testing.T) {
	hub := NewHub()
	hub.maxSubs = 1
	srv := httptest.NewServer(newMux(hub, &pollHealth{}, time.Second, nil))
	defer srv.Close()
	first, err := http.Get(srv.URL + "/events")
	if err != nil || first.StatusCode != 200 {
		t.Fatalf("first stream: %v %v", err, first)
	}
	defer first.Body.Close()
	second, err := http.Get(srv.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Body.Close()
	if second.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("second stream got %d, want 503", second.StatusCode)
	}
}

func gitFixture(t *testing.T, home, name, head string) string {
	t.Helper()
	repo := filepath.Join(home, "src", name)
	write(t, filepath.Join(repo, ".git", "HEAD"), head)
	write(t, filepath.Join(repo, ".git", "config"), "[remote \"origin\"]\n\turl = https://git.example.test/acme/"+name+".git\n")
	write(t, filepath.Join(repo, ".git", "refs", "heads", "main"), strings.Repeat("ab", 20)+"\n")
	return repo
}

func TestReadRepoHardening(t *testing.T) {
	home := t.TempDir()

	repo := gitFixture(t, home, "ok", "ref: refs/heads/main\n")
	if r := readRepo(home, repo); r.Branch != "main" || r.Commit != strings.Repeat("ab", 4)+"a" || r.Name != "ok" {
		t.Errorf("normal repo: %+v", r)
	}

	// Traversal and junk in HEAD never become a path read or a branch name.
	for _, head := range []string{"ref: refs/heads/../../../etc/passwd\n", "ref: ../../etc/passwd\n", "ref: refs/heads/a b\n", "ref: refs/tags/x\n"} {
		bad := gitFixture(t, home, "bad", head)
		if r := readRepo(home, bad); r.Branch != "" || r.Commit != "" {
			t.Errorf("HEAD %q accepted: %+v", head, r)
		}
	}

	// A FIFO where HEAD or config should be must not block.
	fr := gitFixture(t, home, "fifo", "")
	os.Remove(filepath.Join(fr, ".git", "HEAD"))
	if err := syscall.Mkfifo(filepath.Join(fr, ".git", "HEAD"), 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan RepoInfo, 1)
	go func() { done <- readRepo(home, fr) }()
	select {
	case r := <-done:
		if r.Branch != "" {
			t.Errorf("FIFO HEAD read: %+v", r)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("readRepo blocked on a FIFO")
	}

	// A symlinked HEAD is refused.
	sl := gitFixture(t, home, "sym", "ref: refs/heads/main\n")
	secret := filepath.Join(t.TempDir(), "elsewhere")
	os.WriteFile(secret, []byte("ref: refs/heads/main\n"), 0o644)
	os.Remove(filepath.Join(sl, ".git", "HEAD"))
	os.Symlink(secret, filepath.Join(sl, ".git", "HEAD"))
	if r := readRepo(home, sl); r.Branch != "" {
		t.Errorf("symlinked HEAD followed: %+v", r)
	}

	// A worktree whose gitdir points outside home is not believed.
	outside := t.TempDir()
	write(t, filepath.Join(outside, "HEAD"), "ref: refs/heads/main\n")
	wt := filepath.Join(home, "src", "wt")
	write(t, filepath.Join(wt, ".git"), "gitdir: "+outside+"\n")
	if r := readRepo(home, wt); r.Root != "" {
		t.Errorf("gitdir outside home accepted: %+v", r)
	}
	// ... and a repo outside home is not read at all.
	other := gitFixture(t, outside, "stray", "ref: refs/heads/main\n")
	if r := readRepo(home, other); r.Root != "" {
		t.Errorf("repo outside home read: %+v", r)
	}
	// A .git symlinked to a directory outside home is refused.
	link := filepath.Join(home, "src", "linked")
	os.MkdirAll(link, 0o755)
	os.Symlink(filepath.Join(other, ".git"), filepath.Join(link, ".git"))
	if r := readRepo(home, link); r.Root != "" {
		t.Errorf("symlinked .git accepted: %+v", r)
	}
}
