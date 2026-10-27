package upstream

import (
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
)

func snapOf(host string, sessions ...board.SessionView) *board.Snapshot {
	return &board.Snapshot{Host: host, Sessions: sessions, Counts: board.Counts{Live: len(sessions)},
		Cost: board.BoardCost{PricedAsOf: "2026-01-01"}}
}

func sv(id string) board.SessionView {
	return board.SessionView{ID: id, Short: board.ShortID(id), Live: true}
}

var now = time.Unix(1000, 0)

func TestMergePrefixesOnlyCollidingIDsAndRewritesReferences(t *testing.T) {
	a := snapOf("alpha", sv("dup-00000001"), sv("only-a-0001"))
	a.Links = []board.LinkView{{From: "dup-00000001", To: "only-a-0001", At: 5}}
	a.Branches = []board.BranchView{{Repo: "r", Branch: "b", Sessions: []string{"dup-00000001"}}}
	a.Log = []board.LogView{{At: 1, Session: "dup-00000001"}}
	b := snapOf("beta", sv("dup-00000001"))
	m := Merge("hub", nil, []Part{
		{Name: "alpha", State: StateLive, Snap: a}, {Name: "beta", State: StateLive, Snap: b}}, now)

	got := map[string]bool{}
	for _, s := range m.Sessions {
		got[s.ID] = true
	}
	for _, id := range []string{"alpha:dup-00000001", "beta:dup-00000001", "only-a-0001"} {
		if !got[id] {
			t.Errorf("missing %s in %v", id, got)
		}
	}
	if l := m.Links[0]; l.From != "alpha:dup-00000001" || l.To != "only-a-0001" {
		t.Errorf("link %+v", l)
	}
	if m.Branches[0].Sessions[0] != "alpha:dup-00000001" || m.Branches[0].Host != "alpha" || m.Log[0].Session != "alpha:dup-00000001" {
		t.Errorf("branch %+v log %+v", m.Branches[0], m.Log[0])
	}
	if a.Sessions[0].ID != "dup-00000001" || a.Links[0].From != "dup-00000001" {
		t.Error("Merge changed the stored snapshot")
	}
	if m.Counts.Live != 3 || m.Cost.PricedAsOf != "2026-01-01" {
		t.Errorf("counts %+v cost %+v", m.Counts, m.Cost)
	}
}

func TestMergeResolvesExternalLinksAcrossHosts(t *testing.T) {
	a := snapOf("alpha", sv("aaaaaaaa-1111"))
	a.Links = []board.LinkView{
		{From: "aaaaaaaa-1111", To: "bbbbbbbb-2222", ToLabel: "bbbbbbbb-2222", External: true, At: 1},
		{From: "aaaaaaaa-1111", To: "nobody-anywhere", ToLabel: "nobody-anywhere", External: true, At: 2},
	}
	b := snapOf("beta", sv("bbbbbbbb-2222"))
	m := Merge("hub", a, []Part{{Name: "beta", State: StateLive, Snap: b}}, now)
	if l := m.Links[0]; l.External || l.To != "bbbbbbbb-2222" || l.ToLabel != "bbbbbbbb" {
		t.Errorf("not resolved: %+v", l)
	}
	if !m.Links[1].External {
		t.Errorf("unknown target resolved: %+v", m.Links[1])
	}
}

func TestMergeStalePartKeepsSessionsButNotCounts(t *testing.T) {
	a := snapOf("alpha", sv("a-00000001"))
	a.Accounts = []board.AccountView{{Key: "k", Slots: []string{"s1"}, Sessions: 1, Live: 1}}
	b := snapOf("beta", sv("b-00000001"))
	b.Accounts = []board.AccountView{{Key: "k", Slots: []string{"s1", "s2"}, Sessions: 1, Live: 1}}
	m := Merge("hub", nil, []Part{{Name: "alpha", State: StateLive, Snap: a},
		{Name: "beta", State: StateStale, Snap: b}, {Name: "gone", State: StateDown, Err: "HTTP 500"}}, now)
	if len(m.Sessions) != 2 || !m.Sessions[1].Stale || m.Sessions[0].Stale || m.Counts.Live != 1 {
		t.Errorf("sessions %+v counts %+v", m.Sessions, m.Counts)
	}
	acc := m.Accounts[0]
	if len(m.Accounts) != 1 || acc.Sessions != 2 || acc.Live != 1 || len(acc.Slots) != 2 {
		t.Errorf("accounts %+v", m.Accounts)
	}
	if len(m.Upstreams) != 3 || m.Upstreams[2].State != StateDown || m.Upstreams[2].Error != "HTTP 500" {
		t.Errorf("upstreams %+v", m.Upstreams)
	}
}
