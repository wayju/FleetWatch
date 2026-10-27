// Package hookfile is a signal source for agents that write the documented
// session state file (schema/session-state.v1.json; see ADR-010): one JSON file per
// session under a directory, written by the Claude Code plugin in
// plugin/claude-code/ or by any other tool that follows the schema.
//
// It is an Observer only. It discovers no processes (Sessions returns none);
// it joins a state file to a session the other runtimes and agents already
// found, by session id first and by pid second, and answers with a RankNative
// state signal that expires, so a crashed session's last write stops winning.
// It never reads /proc and never writes.
package hookfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/fsafe"
	"github.com/wayju/fleetwatch/internal/host"
)

const (
	maxFiles = 512
	maxBytes = 16 << 10
	// SchemaVersion is the only schema this reader accepts.
	SchemaVersion = 1
)

// Config is where the source finds its files.
type Config struct {
	Dir   string        // directory of <session_id>.json files
	Fresh time.Duration // how long a write stays authoritative
}

// Runtime is the hookfile signal source.
type Runtime struct{ cfg Config }

// New builds the source.
func New(cfg Config) *Runtime { return &Runtime{cfg: cfg} }

// Name is the runtime's config type.
func (*Runtime) Name() string { return "hookfile" }

// Sessions reports nothing: the source only annotates sessions found elsewhere.
func (*Runtime) Sessions(time.Time, host.Claimer) []host.Session { return nil }

// File is the state file's content.
type File struct {
	Schema     int    `json:"schema"`
	SessionID  string `json:"session_id"`
	PID        int    `json:"pid"`
	State      string `json:"state"`
	Reason     string `json:"reason"`
	HookEvent  string `json:"hook_event"`
	Pane       string `json:"pane"`
	Cwd        string `json:"cwd"`
	ObservedAt string `json:"observed_at"`
}

// states are the board vocabulary a file may name.
var states = map[string]bool{"working": true, "waiting": true, "approval": true, "question": true, "idle": true}

// read indexes the valid files by session id and by pid (newest write wins a
// shared pid). A file is refused unless it is a regular file under the size
// cap, carries schema 1 and a known state, and its name is its session id.
func (r *Runtime) read() (bySID map[string]File, byPID map[int]File) {
	bySID, byPID = map[string]File{}, map[int]File{}
	ents, err := os.ReadDir(r.cfg.Dir)
	if err != nil {
		return
	}
	n := 0
	for _, e := range ents {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") || !e.Type().IsRegular() {
			continue
		}
		if n++; n > maxFiles {
			break
		}
		b, err := fsafe.ReadSmall(filepath.Join(r.cfg.Dir, name), maxBytes)
		if err != nil {
			continue
		}
		var f File
		if json.Unmarshal(b, &f) != nil || f.Schema != SchemaVersion || !states[f.State] ||
			f.SessionID != strings.TrimSuffix(name, ".json") || observed(f).IsZero() {
			continue
		}
		bySID[f.SessionID] = f
		if f.PID > 0 {
			if prev, ok := byPID[f.PID]; !ok || observed(f).After(observed(prev)) {
				byPID[f.PID] = f
			}
		}
	}
	return
}

func observed(f File) time.Time {
	t, err := time.Parse(time.RFC3339Nano, f.ObservedAt)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Observe gives each bound session its file's state and pane. A file written
// before the process started belongs to a previous owner of the pid.
func (r *Runtime) Observe(now time.Time, bs []host.Bound) []host.Observation {
	out := make([]host.Observation, len(bs))
	bySID, byPID := r.read()
	taken := map[string]bool{} // bs is newest process first: it takes a shared session id
	for i, b := range bs {
		f, ok := bySID[b.SessionID]
		if !ok || b.SessionID == "" || taken[f.SessionID] {
			f, ok = byPID[b.PID]
		}
		at := observed(f)
		if !ok || taken[f.SessionID] || at.Before(b.Start.Add(-time.Second)) {
			continue
		}
		taken[f.SessionID] = true
		out[i].Address = f.Pane
		out[i].States = []host.StateSignal{{State: f.State, Reason: f.Reason, Source: "hook",
			Rank: host.RankNative, ObservedAt: at, FreshUntil: at.Add(r.cfg.Fresh)}}
	}
	return out
}
