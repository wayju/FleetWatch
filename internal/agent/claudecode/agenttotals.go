package claudecode

// Per-session subagent totals from <session>/subagents/agent-<id>.meta.json,
// which Claude Code keeps for every subagent a session ever launched (a busy
// session has well over a hundred). Unlike AgentsOf, nothing here expires: the
// card keeps the delegation count after the individual agents leave the list.

import (
	"encoding/json"
	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// metaEntry is one meta file as last read. The meta is re-read only when its
// size or mtime moves (finished flips in place), so a steady-state poll costs
// a stat per agent and no reads. A finished agent's transcript mtime is fixed
// once seen; an unfinished one is re-stat'd each poll.
type metaEntry struct {
	size     int64
	mod      time.Time
	finished bool
	subMod   time.Time // transcript mtime; zero when the transcript is missing
}

type totalsCache struct {
	entries map[string]*metaEntry // by meta path
	seen    map[string]bool       // meta paths read this Collect pass
}

// begin starts a pass; prune drops every entry the pass did not touch, so a
// finished session or a deleted meta never lingers in a long-running service.
func (tc *totalsCache) begin() { tc.seen = map[string]bool{} }

func (tc *totalsCache) prune() {
	for p := range tc.entries {
		if !tc.seen[p] {
			delete(tc.entries, p)
		}
	}
}

const maxMetaBytes = 64 << 10

// agentTotals counts the subagents under sessionPath's sibling directory and
// returns the ids of those their meta marks finished. Running means unfinished
// with a transcript touched within AgentStall; an unfinished agent quiet for
// longer was abandoned and counts only in Total.
func (a *Agent) agentTotals(sessionPath string, now time.Time) (board.AgentTotals, map[string]bool) {
	var tot board.AgentTotals
	if sessionPath == "" {
		return tot, nil
	}
	dir := filepath.Join(strings.TrimSuffix(sessionPath, ".jsonl"), "subagents")
	metas, err := filepath.Glob(filepath.Join(dir, "agent-*.meta.json"))
	if err != nil || len(metas) == 0 {
		return tot, nil
	}
	if a.totals.entries == nil {
		a.totals.entries = map[string]*metaEntry{}
	}
	if a.totals.seen == nil {
		a.totals.seen = map[string]bool{}
	}
	finished := map[string]bool{}
	for _, mp := range metas {
		st, err := os.Lstat(mp)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		a.totals.seen[mp] = true
		e := a.metaFor(mp, st)
		id := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(mp), "agent-"), ".meta.json")
		tot.Total++
		if e.finished {
			finished[id] = true
			at := e.subMod
			if at.IsZero() {
				at = e.mod
			}
			if u := at.Unix(); u > tot.LastFinishedAt {
				tot.LastFinishedAt = u
			}
		} else if !e.subMod.IsZero() && now.Sub(e.subMod) <= AgentStall {
			tot.Running++
		}
	}
	return tot, finished
}

// metaFor returns the cached entry for a meta file, re-reading it when it
// changed, and refreshes the transcript mtime where that can still move.
func (a *Agent) metaFor(mp string, st os.FileInfo) *metaEntry {
	e := a.totals.entries[mp]
	if e == nil || e.size != st.Size() || !e.mod.Equal(st.ModTime()) {
		e = &metaEntry{size: st.Size(), mod: st.ModTime()}
		if b, err := fsafe.ReadSmall(mp, maxMetaBytes); err == nil {
			var m struct {
				Finished bool `json:"finished"`
			}
			_ = json.Unmarshal(b, &m)
			e.finished = m.Finished
		}
		a.totals.entries[mp] = e
	}
	if e.subMod.IsZero() || !e.finished {
		if sst, err := os.Lstat(strings.TrimSuffix(mp, ".meta.json") + ".jsonl"); err == nil {
			e.subMod = sst.ModTime()
		}
	}
	return e
}
