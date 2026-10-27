package upstream

import "github.com/wayju/fleetwatch/internal/board"

// mergeAccounts folds the hosts' account rows by key. The collector hashes the
// email, so one login on two hosts shares a key. A stale host's sessions still
// count as sessions but not as live.
func mergeAccounts(out *board.Snapshot, ps []*part) {
	at := map[string]int{}
	for _, p := range ps {
		for _, a := range p.snap.Accounts {
			i, ok := at[a.Key]
			if !ok {
				a.Slots = append([]string{}, a.Slots...)
				if p.stale {
					a.Live = 0
				}
				at[a.Key] = len(out.Accounts)
				out.Accounts = append(out.Accounts, a)
				continue
			}
			m := &out.Accounts[i]
			m.Sessions += a.Sessions
			if !p.stale {
				m.Live += a.Live
			}
			m.Slots = unionSlots(m.Slots, a.Slots)
			if m.Email == "" {
				m.Email = a.Email
			}
			if m.Name == "" {
				m.Name, m.Org = a.Name, a.Org
			}
		}
	}
}

func unionSlots(have, add []string) []string {
	seen := map[string]bool{}
	for _, s := range have {
		seen[s] = true
	}
	for _, s := range add {
		if !seen[s] {
			have = append(have, s)
			seen[s] = true
		}
	}
	return have
}
