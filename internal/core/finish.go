package core

// The board-level views derived from the per-session ones: ordering, counts,
// accounts, branches in flight, who messaged whom, and the merged event log.

import (
	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/board"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const maxLog = 60

var stateRank = map[string]int{
	"approval": 0, "question": 0, "waiting": 1, "working": 2, "delegating": 2, "starting": 3, "idle": 4, "unknown": 5, "ended": 6,
}

func finish(s *Snapshot, srcs map[string]agent.Session, showText bool) {
	sortSessions(s.Sessions)

	accounts := map[string]*AccountView{}
	branches := map[string]*BranchView{}
	ids := sessionIDs(s.Sessions)
	var links []LinkView
	for i := range s.Sessions {
		v := &s.Sessions[i]
		countSession(&s.Counts, v)
		addAccount(accounts, v)
		addBranch(branches, v)
		links = append(links, resolveMessages(v, ids, showText)...)
	}
	s.Links = dedupeLinks(append(links, agentLinks(s.Sessions, ids, showText)...))

	s.Accounts = sortedAccounts(accounts)
	s.Branches = sortedBranches(branches, s.Sessions)
	s.Log = mergeLog(srcs, showText)

	if s.Sessions == nil {
		s.Sessions = []SessionView{}
	}
	if s.Accounts == nil {
		s.Accounts = []AccountView{}
	}
	if s.Branches == nil {
		s.Branches = []BranchView{}
	}
	if s.Links == nil {
		s.Links = []LinkView{}
	}
	if s.Log == nil {
		s.Log = []LogView{}
	}
}

// sortSessions orders the board: live first, then by state urgency, newest
// activity, and finally id for stability.
func sortSessions(ss []SessionView) {
	sort.SliceStable(ss, func(i, j int) bool {
		a, b := ss[i], ss[j]
		if a.Live != b.Live {
			return a.Live
		}
		if stateRank[a.State] != stateRank[b.State] {
			return stateRank[a.State] < stateRank[b.State]
		}
		if a.LastActivity != b.LastActivity {
			return a.LastActivity > b.LastActivity
		}
		return a.ID < b.ID
	})
}

// countSession folds one session into the headline counts.
func countSession(c *Counts, v *SessionView) {
	if v.Live {
		c.Live++
		switch v.State {
		case "working", "delegating":
			c.Working++
		case "waiting":
			c.Waiting++
		case "approval", "question":
			c.Approval++
		}
	}
	for _, a := range v.Agents {
		if a.Status == "running" {
			c.AgentsRunning++
		}
	}
}

// addAccount tallies a session under its account, creating the view on first sight.
func addAccount(accounts map[string]*AccountView, v *SessionView) {
	acc := accounts[v.AccountKey]
	if acc == nil {
		acc = &AccountView{Key: v.AccountKey, Email: v.Account.Email, Name: v.Account.Name, Org: v.Account.Org, Slots: []string{}}
		accounts[v.AccountKey] = acc
	}
	acc.Sessions++
	if v.Live {
		acc.Live++
	}
	slot := v.Slot
	if slot == "" {
		slot = "default"
	}
	if !contains(acc.Slots, slot) {
		acc.Slots = append(acc.Slots, slot)
	}
}

// addBranch groups a not-yet-ended session under its repo and branch.
func addBranch(branches map[string]*BranchView, v *SessionView) {
	if v.Branch == "" || v.State == "ended" {
		return
	}
	repo := v.Repo.Slug()
	if repo == "" {
		repo = v.Repo.Name
	}
	key := repo + "\x00" + v.Branch
	b := branches[key]
	if b == nil {
		b = &BranchView{Repo: repo, Branch: v.Branch, Sessions: []string{}}
		branches[key] = b
	}
	if b.Version == "" {
		b.Version = v.Repo.Version
	}
	if b.Commit == "" {
		b.Commit = v.Repo.Commit
	}
	b.Sessions = append(b.Sessions, v.ID)
}

// resolveMessages points a session's outgoing messages at board sessions where
// the recipient resolves, marks the rest external, and returns copies.
func resolveMessages(v *SessionView, ids map[string]string, showText bool) []LinkView {
	var out []LinkView
	for k := range v.Messages {
		m := &v.Messages[k]
		if m.ToAgent == "" {
			if id, ok := resolveSession(ids, m.To); ok {
				m.To, m.ToLabel = id, board.ShortID(id)
			} else {
				markExternal(m, showText)
			}
		}
		out = append(out, *m)
	}
	return out
}

func sortedAccounts(accounts map[string]*AccountView) []AccountView {
	var out []AccountView
	for _, a := range accounts {
		sort.Strings(a.Slots)
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Live != out[j].Live {
			return out[i].Live > out[j].Live
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// sortedBranches flags branches with more than one live session and orders
// them by repo, then branch.
func sortedBranches(branches map[string]*BranchView, sessions []SessionView) []BranchView {
	live := map[string]bool{}
	for _, v := range sessions {
		live[v.ID] = v.Live
	}
	var out []BranchView
	for _, b := range branches {
		n := 0
		for _, id := range b.Sessions {
			if live[id] {
				n++
			}
		}
		b.Shared = n > 1
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Branch < out[j].Branch
	})
	return out
}

// mergeLog interleaves every session's event log in time order and keeps the
// newest maxLog entries.
func mergeLog(srcs map[string]agent.Session, showText bool) []LogView {
	var log []LogView
	for id, src := range srcs {
		for _, lv := range src.Log(showText) {
			lv.Session, lv.Short = id, board.ShortID(id)
			log = append(log, lv)
		}
	}
	sort.SliceStable(log, func(i, j int) bool {
		a, b := log[i], log[j]
		if a.At != b.At {
			return a.At < b.At
		}
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Text < b.Text
	})
	if len(log) > maxLog {
		log = log[len(log)-maxLog:]
	}
	return log
}

const (
	pidKey  = "pid:"  // ids key for a live process, matched by a uds: socket address
	nameKey = "name:" // ids key for a peer's registry name; "" marks an ambiguous name
)

// sessionIDs maps every way a message may name a session to its id: full id,
// short id, and for live sessions the slot, pid and registry name.
func sessionIDs(sessions []SessionView) map[string]string {
	ids := map[string]string{}
	for _, v := range sessions {
		ids[v.ID] = v.ID
		ids[v.Short] = v.ID
		if !v.Live {
			continue
		}
		if v.Slot != "" {
			ids[v.Slot] = v.ID
		}
		ids[pidKey+strconv.Itoa(v.PID)] = v.ID
		// Only the registry name: SendMessage addresses peers by it, and it
		// already carries a user-set title. A bare title is free text.
		if v.PeerName != "" {
			addPeerName(ids, v.PeerName, v.ID)
		}
	}
	return ids
}

// addPeerName registers a live session's registry name. Two live sessions
// sharing one name make it ambiguous, so the key is blanked rather than guessed.
func addPeerName(ids map[string]string, name, id string) {
	if prev, ok := ids[nameKey+name]; ok && prev != id {
		ids[nameKey+name] = ""
		return
	}
	ids[nameKey+name] = id
}

// resolveSession maps a message's recipient to a session on the board: an
// exact id, a slot name, a uds: socket address (by pid), a unique peer name or
// registry name, or an unambiguous id prefix of at least 8 chars.
func resolveSession(ids map[string]string, to string) (string, bool) {
	to = strings.TrimPrefix(to, "session:")
	if strings.HasPrefix(to, pidKey) || strings.HasPrefix(to, nameKey) {
		return "", false // reserved key namespaces are not recipients
	}
	if id, ok := ids[to]; ok {
		return id, true
	}
	if sock, ok := strings.CutPrefix(to, "uds:"); ok {
		pid, ok := strings.CutSuffix(filepath.Base(sock), ".sock")
		id := ids[pidKey+pid]
		return id, ok && id != ""
	}
	if id := ids[nameKey+to]; id != "" {
		return id, true
	}
	if _, ambiguous := ids[nameKey+to]; ambiguous {
		return "", false
	}
	if len(to) < 8 {
		return "", false
	}
	found := ""
	for k, id := range ids {
		if strings.Contains(k, ":") {
			continue
		}
		if strings.HasPrefix(k, to) || strings.HasPrefix(to, k) && len(k) >= 8 {
			if found != "" && found != id {
				return "", false
			}
			found = id
		}
	}
	return found, found != ""
}

func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
