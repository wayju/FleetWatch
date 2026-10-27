package claudecode

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/host"
)

// agentIn is an agent over <home>/.claude plus one extra slot dir.
func agentIn(t *testing.T) (*Agent, string) {
	home := t.TempDir()
	slot := filepath.Join(home, ".claude-alt", "work")
	a := New(Config{
		Dirs:        []host.ConfigDir{{Path: filepath.Join(home, ".claude")}},
		ExtraDirs:   func() []host.ConfigDir { return []host.ConfigDir{{Path: slot, Label: "work"}} },
		Window:      2 * time.Hour,
		RescanEvery: time.Second,
	})
	return a, home
}

func TestClaims(t *testing.T) {
	a := New(Config{})
	cmd := func(s string) func() string { return func() string { return s } }
	if !a.Claims("claude", func() string { t.Fatal("claude must not read the command line"); return "" }) {
		t.Error("claude")
	}
	if !a.Claims("node", cmd("node /x/@anthropic-ai/claude-code/cli.js")) || a.Claims("node", cmd("node server.js")) || a.Claims("bash", cmd("claude")) {
		t.Error("node and others")
	}
}

func TestBindExactReadsTheSessionFileOfTheProcessConfigDir(t *testing.T) {
	a, home := agentIn(t)
	write(t, filepath.Join(home, ".claude", "sessions", "10.json"), `{"sessionId":"s-default","name":"p-default"}`)
	write(t, filepath.Join(home, ".claude-alt", "work", "sessions", "11.json"), `{"sessionId":"s-work"}`)
	write(t, filepath.Join(home, ".claude", "sessions", "12.json"), `{"name":"only-a-name"}`)
	if b, ok := a.Bind(host.Session{PID: 10}, agent.BindExact, nil); !ok || b.SessionID != "s-default" || b.Peer != "p-default" || b.How != "session-file" {
		t.Fatalf("default dir: %+v %v", b, ok)
	}
	if b, ok := a.Bind(host.Session{PID: 11, ConfigDir: filepath.Join(home, ".claude-alt", "work")}, agent.BindExact, nil); !ok || b.SessionID != "s-work" {
		t.Fatalf("slot dir: %+v %v", b, ok)
	}
	if b, ok := a.Bind(host.Session{PID: 12}, agent.BindExact, nil); !ok || b.SessionID != "" || b.Peer != "only-a-name" {
		t.Fatalf("a name without a session id still names the peer: %+v %v", b, ok)
	}
	if _, ok := a.Bind(host.Session{PID: 99}, agent.BindExact, nil); ok {
		t.Error("no session file, no binding")
	}
}

func transcriptAt(t *testing.T, dir, id string, lastSec int) {
	write(t, filepath.Join(dir, "projects", "-p", id+".jsonl"), userText(lastSec, "hi")+assistantEnd(lastSec+1, "ok"))
}

func TestBindGuessPicksTheNewestUnclaimedInTheSameLabelAndTiesGoToTheSmallerID(t *testing.T) {
	a, home := agentIn(t)
	def, slot := filepath.Join(home, ".claude"), filepath.Join(home, ".claude-alt", "work")
	transcriptAt(t, def, "bbb", 10)
	transcriptAt(t, def, "aaa", 10) // same activity as bbb: the smaller id wins
	transcriptAt(t, slot, "zzz", 50)
	transcriptAt(t, def, "old", -3600)
	a.Tail(t0.Add(time.Minute), map[string]bool{})
	start := t0.Add(-30 * time.Second)

	b, ok := a.Bind(host.Session{Start: start, Label: "work"}, agent.BindGuess, map[string]bool{})
	if !ok || b.SessionID != "zzz" || b.How != "guess" {
		t.Fatalf("work slot: %+v %v", b, ok)
	}
	b, ok = a.Bind(host.Session{Start: start}, agent.BindGuess, map[string]bool{"zzz": true})
	if !ok || b.SessionID != "aaa" {
		t.Fatalf("tie: %+v %v", b, ok)
	}
	if _, ok := a.Bind(host.Session{Start: start, Label: "other"}, agent.BindGuess, map[string]bool{"aaa": true, "bbb": true}); ok {
		t.Error("a label that matches nothing must not take a labelled transcript, and old ones are before the start")
	}
}

func TestRecentListsUnlinkedIdleSessionsByID(t *testing.T) {
	a, home := agentIn(t)
	def := filepath.Join(home, ".claude")
	transcriptAt(t, def, "ccc", -600)
	transcriptAt(t, def, "aaa", -300)
	transcriptAt(t, def, "bbb", -400)
	transcriptAt(t, def, "ancient", -7200)
	a.Tail(t0, map[string]bool{})
	got := a.Recent(t0, 30*time.Minute, map[string]bool{"bbb": true})
	if len(got) != 2 || got[0] != "aaa" || got[1] != "ccc" {
		t.Fatalf("%v", got)
	}
	s := a.Session("aaa")
	if s == nil || s.ID() != "aaa" || s.Label() != "" || s.ConfigDir() != "" {
		t.Fatalf("default dir session: %+v", s)
	}
	if a.Session("missing") != nil {
		t.Error("unknown id")
	}
}

func TestSessionLabelAndConfigDirNameTheSlotThatHoldsIt(t *testing.T) {
	a, home := agentIn(t)
	slot := filepath.Join(home, ".claude-alt", "work")
	transcriptAt(t, slot, "sss", 5)
	a.Tail(t0, map[string]bool{})
	s := a.Session("sss")
	if s == nil || s.Label() != "work" || s.ConfigDir() != slot {
		t.Fatalf("%+v", s)
	}
}

func TestAccountDefaultSitsBesideTheConfigDir(t *testing.T) {
	a, home := agentIn(t)
	write(t, filepath.Join(home, ".claude.json"), `{"oauthAccount":{"emailAddress":"d@x.example","displayName":"D"}}`)
	write(t, filepath.Join(home, ".claude-alt", "work", ".claude.json"), `{"oauthAccount":{"emailAddress":"w@x.example"}}`)
	if got := a.Account(""); got.Email != "d@x.example" || got.Name != "D" {
		t.Errorf("default: %+v", got)
	}
	if got := a.Account(filepath.Join(home, ".claude-alt", "work")); got.Email != "w@x.example" {
		t.Errorf("slot: %+v", got)
	}
	if got := a.Account(filepath.Join(home, "nope")); got.Email != "" {
		t.Errorf("missing: %+v", got)
	}
}
