package upstream

// Merge folds the boards of several hosts into one Snapshot (see ADR-008). Stored
// snapshots are shared between merges, so every slice that changes is copied.

import (
	"sort"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
)

const maxLog = 60 // the collector's own cap, so the merged log is the same size

type part struct {
	label string
	snap  *board.Snapshot
	stale bool
	ids   map[string]string // original session id -> merged id, for this host
}

// Merge builds the hub's board. An upstream's sessions are keyed by its
// configured name, not the hostname it reports, which inside a container is
// often an id. local is this instance's own host (nil when
// the hub does not collect); parts come in config order after it.
func Merge(host string, local *board.Snapshot, parts []Part, now time.Time) board.Snapshot {
	var ps []*part
	if local != nil {
		ps = append(ps, &part{label: labelOf(local, host), snap: local})
	}
	views := []board.UpstreamView{}
	for _, p := range parts {
		views = append(views, board.UpstreamView{Name: p.Name, State: p.State, LastSeen: board.Unix(p.LastSeen), Error: p.Err})
		if p.Snap != nil {
			ps = append(ps, &part{label: p.Name, snap: p.Snap, stale: p.State != StateLive})
		}
	}
	assignIDs(ps)
	out := board.Snapshot{GeneratedAt: now.Unix(), Host: host, Upstreams: views,
		Sessions: []board.SessionView{}, Accounts: []board.AccountView{}, Branches: []board.BranchView{},
		Worktrees: []board.WorktreeView{}, Links: []board.LinkView{}, Log: []board.LogView{}}
	for _, p := range ps {
		p.addSessions(&out)
	}
	resolve := newResolver(ps)
	for _, p := range ps {
		p.addLinks(&out, resolve)
		p.addBoardLists(&out)
	}
	for i := range out.Sessions {
		out.Sessions[i].Messages = resolveAll(out.Sessions[i].Messages, resolve)
	}
	mergeAccounts(&out, ps)
	sortLists(&out)
	asOf := ""
	for _, p := range ps {
		if asOf == "" {
			asOf = p.snap.Cost.PricedAsOf
		}
	}
	out.Cost = board.TotalCost(out.Sessions, asOf)
	return out
}

func labelOf(s *board.Snapshot, fallback string) string {
	if s.Host != "" {
		return s.Host
	}
	return fallback
}

// assignIDs prefixes a session id with its host only where two hosts hold the
// same id, so a single-host id stays as the host knows it.
func assignIDs(ps []*part) {
	count := map[string]int{}
	for _, p := range ps {
		for _, s := range p.snap.Sessions {
			count[s.ID]++
		}
	}
	for _, p := range ps {
		p.ids = map[string]string{}
		for _, s := range p.snap.Sessions {
			p.ids[s.ID] = s.ID
			if count[s.ID] > 1 {
				p.ids[s.ID] = p.label + ":" + s.ID
			}
		}
	}
}

func (p *part) id(orig string) string {
	if n, ok := p.ids[orig]; ok {
		return n
	}
	return orig
}

func (p *part) addSessions(out *board.Snapshot) {
	for _, s := range p.snap.Sessions {
		s.ID = p.id(s.ID)
		s.Host = p.label
		s.Stale = p.stale
		s.Messages = p.remapLinks(s.Messages)
		out.Sessions = append(out.Sessions, s)
	}
}

func (p *part) addBoardLists(out *board.Snapshot) {
	for _, b := range p.snap.Branches {
		b.Host, b.Sessions = p.label, p.mapIDs(b.Sessions)
		out.Branches = append(out.Branches, b)
	}
	for _, w := range p.snap.Worktrees {
		w.Host = p.label
		ws := make([]board.WorktreeWriter, len(w.Writers))
		for i, x := range w.Writers {
			x.Session = p.id(x.Session)
			ws[i] = x
		}
		w.Writers = ws
		out.Worktrees = append(out.Worktrees, w)
	}
	for _, l := range p.snap.Log {
		l.Session = p.id(l.Session)
		out.Log = append(out.Log, l)
	}
	if !p.stale {
		c, o := p.snap.Counts, &out.Counts
		o.Live, o.Working, o.Waiting, o.Approval = o.Live+c.Live, o.Working+c.Working, o.Waiting+c.Waiting, o.Approval+c.Approval
		o.AgentsRunning, o.Servers, o.ServersStale = o.AgentsRunning+c.AgentsRunning, o.Servers+c.Servers, o.ServersStale+c.ServersStale
	}
}

func (p *part) mapIDs(in []string) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = p.id(id)
	}
	return out
}

// remapLinks copies a host's links with their session ids renamed.
func (p *part) remapLinks(in []board.LinkView) []board.LinkView {
	out := make([]board.LinkView, len(in))
	for i, l := range in {
		if n, ok := p.ids[l.From]; ok {
			l.From = n
		}
		if n, ok := p.ids[l.To]; ok {
			l.To = n
		}
		out[i] = l
	}
	return out
}

func (p *part) addLinks(out *board.Snapshot, r *resolver) {
	out.Links = append(out.Links, resolveAll(p.remapLinks(p.snap.Links), r)...)
}

// resolver finds a session on any host from an id a link named.
type resolver struct{ ids map[string][]string }

func newResolver(ps []*part) *resolver {
	r := &resolver{ids: map[string][]string{}}
	for _, p := range ps {
		for orig, merged := range p.ids {
			r.ids[orig] = append(r.ids[orig], merged)
		}
	}
	return r
}

// find returns the merged id for a link target that no single host could
// place: an exact id, or an unambiguous prefix of at least 8 characters.
func (r *resolver) find(to string) (string, bool) {
	if to == "" {
		return "", false
	}
	if m := r.ids[to]; len(m) == 1 {
		return m[0], true
	}
	if len(to) < 8 {
		return "", false
	}
	found := ""
	for orig, m := range r.ids {
		if strings.HasPrefix(orig, to) && len(m) == 1 {
			if found != "" && found != m[0] {
				return "", false
			}
			found = m[0]
		}
	}
	return found, found != ""
}

// resolveAll turns external links into internal ones where the target lives
// on another host. The input is already a private copy.
func resolveAll(in []board.LinkView, r *resolver) []board.LinkView {
	out := make([]board.LinkView, len(in))
	copy(out, in)
	for i := range out {
		if !out[i].External {
			continue
		}
		if id, ok := r.find(out[i].To); ok {
			out[i].To, out[i].ToLabel, out[i].External = id, board.ShortID(id), false
		}
	}
	return out
}

func sortLists(out *board.Snapshot) {
	sort.SliceStable(out.Links, func(i, j int) bool { return out.Links[i].At < out.Links[j].At })
	sort.SliceStable(out.Log, func(i, j int) bool { return out.Log[i].At < out.Log[j].At })
	if len(out.Log) > maxLog {
		out.Log = out.Log[len(out.Log)-maxLog:]
	}
	sort.SliceStable(out.Branches, func(i, j int) bool {
		a, b := out.Branches[i], out.Branches[j]
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if a.Branch != b.Branch {
			return a.Branch < b.Branch
		}
		return a.Host < b.Host
	})
}
