package core

// Links whose two ends sit in one session card: the page draws these inside the
// card, so the data must name both ends exactly (card, agent and kind).

import "testing"

const sid = "sess1111-aaaa"

func TestSameCardLinkEndpoints(t *testing.T) {
	cases := []struct {
		name         string
		sub          map[string]string
		main         string
		from, fa, ta string
		kind         string
	}{
		{"agent to sibling by id",
			map[string]string{agA: assistantTool(5, "m1", "SendMessage", map[string]any{"to": agB, "message": "hi"})},
			asyncLaunch(0, "t1", agA) + asyncLaunch(1, "t2", agB), sid, agA, agB, "message"},
		{"agent to sibling by launch name",
			map[string]string{agA: assistantTool(5, "m1", "SendMessage", map[string]any{"to": "demo-b", "message": "hi"})},
			namedLaunch(0, "t1", agA, "demo-a") + namedLaunch(1, "t2", agB, "demo-b"), sid, agA, agB, "message"},
		{"agent to main",
			map[string]string{agA: assistantTool(5, "m1", "SendMessage", map[string]any{"to": "main", "message": "done"})},
			asyncLaunch(0, "t1", agA), sid, agA, "", "message"},
		{"hand-back peer line from an agent to a sibling",
			map[string]string{agB: peerLine(7, agA, true)},
			asyncLaunch(0, "t1", agA) + asyncLaunch(1, "t2", agB), sid, agA, agB, "handback"},
		{"hand-back peer line from a named sibling",
			map[string]string{agB: peerLine(7, "demo-a", true)},
			namedLaunch(0, "t1", agA, "demo-a") + namedLaunch(1, "t2", agB, "demo-b"), sid, agA, agB, "handback"},
		{"session to its own agent",
			nil, asyncLaunch(0, "t1", agA) + sendMsg(3, "m0", agA, "go"), sid, "", agA, "message"},
	}
	for _, c := range cases {
		ls := linksOf(true, sessionOf(sid, c.main, c.sub))
		l := find(ls, c.from, c.fa, sid, c.ta, c.kind)
		if l == nil || l.External {
			t.Errorf("%s: no same-card %s link in %+v", c.name, c.kind, ls)
		}
	}
}

// A hand-back reaches the parent session as an agent-to-main link the page draws
// from the agent box to the card header.
func TestHandbackToMainFromLifeline(t *testing.T) {
	v := sessionOf(sid, asyncLaunch(0, "t1", agA)+peerLine(30, agA, true), nil)
	if find(linksOf(true, v), sid, agA, sid, "", "handback") == nil {
		t.Fatal("hand-back to main missing")
	}
}
