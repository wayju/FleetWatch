package core

import (
	"github.com/wayju/fleetwatch/internal/board"
	"testing"
	"time"
)

const (
	agA = "a39a913ba1b59e565"
	agB = "a11111111111111aa"
)

// sessionOf folds a main transcript and subagent transcripts into a view, as the collector does.
func sessionOf(id string, main string, subs map[string]string) SessionView {
	t := newTranscript(id)
	applyAll(t, main)
	st := map[string]*Transcript{}
	for k, body := range subs {
		s := newTranscript(k)
		applyAll(s, body)
		st[k] = s
	}
	ag := agentsOf(t0.Add(time.Minute), nil, t, st)
	return SessionView{ID: id, Short: board.ShortID(id), Live: true, Agents: ag, Messages: mainLinks(id, t, ag, func(s string) string { return s })}
}

func linksOf(showText bool, vs ...SessionView) []LinkView {
	s := &Snapshot{Sessions: vs}
	finish(s, nil, showText)
	return s.Links
}

func find(ls []LinkView, from, fa, to, ta, kind string) *LinkView {
	for i := range ls {
		if l := &ls[i]; l.From == from && l.FromAgent == fa && l.To == to && l.ToAgent == ta && l.Kind == kind {
			return l
		}
	}
	return nil
}

func TestMainToOwnAgentAndAgentToMain(t *testing.T) {
	sub := assistantTool(5, "m1", "SendMessage", map[string]any{"to": "main", "message": "done"})
	v := sessionOf("sess1111-aaaa", asyncLaunch(0, "t1", agA)+sendMsg(3, "m0", agA, "go"), map[string]string{agA: sub})
	ls := linksOf(true, v)
	if l := find(ls, "sess1111-aaaa", "", "sess1111-aaaa", agA, "message"); l == nil || l.ToLabel != "code-reviewer a39a913" || l.External {
		t.Fatalf("main->agent %+v", ls)
	}
	if l := find(ls, "sess1111-aaaa", agA, "sess1111-aaaa", "", "message"); l == nil || l.Preview != "done" {
		t.Fatalf("agent->main %+v", ls)
	}
	if v.Messages[0].ToAgent != agA || v.Messages[0].To != "sess1111-aaaa" {
		t.Fatalf("session message %+v", v.Messages[0])
	}
}

func TestSiblingAndChildHandbackIntoAgent(t *testing.T) {
	subA := assistantTool(5, "m1", "SendMessage", map[string]any{"to": agB, "message": "hello"})
	subB := peerLine(7, agA, true)
	v := sessionOf("sess1111-aaaa", asyncLaunch(0, "t1", agA)+asyncLaunch(1, "t2", agB), map[string]string{agA: subA, agB: subB})
	ls := linksOf(true, v)
	if find(ls, "sess1111-aaaa", agA, "sess1111-aaaa", agB, "message") == nil {
		t.Fatalf("sibling %+v", ls)
	}
	if find(ls, "sess1111-aaaa", agA, "sess1111-aaaa", agB, "handback") == nil {
		t.Fatalf("child handback %+v", ls)
	}
}

func TestLifelineHandbackAndCrossSessionAgent(t *testing.T) {
	v1 := sessionOf("sess1111-aaaa", asyncLaunch(0, "t1", agA)+peerLine(30, agA, true), nil)
	if l := find(linksOf(true, v1), "sess1111-aaaa", agA, "sess1111-aaaa", "", "handback"); l == nil || l.At != t0.Add(30*time.Second).Unix() {
		t.Fatalf("lifeline handback missing")
	}
	// An agent in session 2 messages an agent of session 1.
	sub := assistantTool(5, "m1", "SendMessage", map[string]any{"to": agA, "message": "hey"})
	v2 := sessionOf("sess2222-bbbb", asyncLaunch(0, "t9", agB), map[string]string{agB: sub})
	ls := linksOf(true, v1, v2)
	if l := find(ls, "sess2222-bbbb", agB, "sess1111-aaaa", agA, "message"); l == nil || l.External {
		t.Fatalf("cross-session %+v", ls)
	}
}

func TestExternalRecipientUnchangedAndShowTextOff(t *testing.T) {
	v := sessionOf("sess1111-aaaa", sendMsg(1, "m1", "somebody-else", "secret plan"), nil)
	if l := linksOf(true, v)[0]; !l.External || l.To != "somebody-else" || l.Kind != "message" {
		t.Fatalf("%+v", l)
	}
	sub := assistantTool(5, "m2", "SendMessage", map[string]any{"to": "outsider", "message": "plan"})
	v2 := sessionOf("sess2222-bbbb", asyncLaunch(0, "t1", agA)+sendMsg(2, "m3", agA, "go"), map[string]string{agA: sub})
	v2.Messages[0].Preview = ""
	for _, l := range linksOf(false, v2) {
		if l.Preview != "" || (l.External && (l.To != "" || l.ToLabel != "external")) {
			t.Fatalf("text leaked %+v", l)
		}
	}
	if l := find(linksOf(false, v2), "sess2222-bbbb", "", "sess2222-bbbb", agA, "message"); l == nil || l.ToLabel == "" {
		t.Fatal("agent label is not free text and must survive")
	}
}

func peerSession(id string, pid int, name, title string, live bool, msgs string) SessionView {
	v := sessionOf(id, msgs, nil)
	v.PID, v.PeerName, v.Title, v.Live = pid, name, title, live
	return v
}

func TestPeerNameAndSocketResolveToSession(t *testing.T) {
	to := peerSession("tgt11111-aaaa", 2609004, "example-stack-c4", "fleet improvements", true, "")
	byName := peerSession("src11111-bbbb", 11, "", "", true, sendMsg(1, "m1", "example-stack-c4", "hi"))
	byUDS := peerSession("src33333-dddd", 13, "", "", true, sendMsg(1, "m3", "uds:/run/user/1000/cc-socks/2609004.sock", "hi"))
	ls := linksOf(true, to, byName, byUDS)
	for _, from := range []string{"src11111-bbbb", "src33333-dddd"} {
		if l := find(ls, from, "", "tgt11111-aaaa", "", "message"); l == nil || l.External {
			t.Fatalf("%s not resolved: %+v", from, ls)
		}
	}
}

// A title is free text; only the registry name (which already carries a
// user-set title) addresses a peer.
func TestBareTitleAndMalformedSocketStayExternal(t *testing.T) {
	to := peerSession("tgt11111-aaaa", 2609004, "example-stack-c4", "fleet improvements", true, "")
	src := peerSession("src11111-bbbb", 3, "", "", true,
		sendMsg(1, "m1", "fleet improvements", "hi")+sendMsg(2, "m2", "uds:/run/user/1000/cc-socks/2609004", "hi"))
	for _, l := range linksOf(true, to, src) {
		if l.From == "src11111-bbbb" && !l.External {
			t.Fatalf("title or malformed socket resolved: %+v", l)
		}
	}
}

func TestAmbiguousPeerNameStaysExternal(t *testing.T) {
	a := peerSession("aaa11111-aaaa", 1, "twin", "", true, "")
	b := peerSession("bbb11111-bbbb", 2, "twin", "", true, "")
	src := peerSession("src11111-cccc", 3, "", "", true, sendMsg(1, "m1", "twin", "hi"))
	for _, l := range linksOf(true, a, b, src) {
		if l.From == "src11111-cccc" && !l.External {
			t.Fatalf("ambiguous name resolved: %+v", l)
		}
	}
}

func TestNonLivePeerNotMatched(t *testing.T) {
	gone := peerSession("old11111-aaaa", 2609004, "gone-peer", "gone title", false, "")
	src := peerSession("src11111-bbbb", 3, "", "", true,
		sendMsg(1, "m1", "gone-peer", "hi")+sendMsg(2, "m2", "uds:/run/x/2609004.sock", "hi")+sendMsg(3, "m3", "pid:2609004", "hi"))
	for _, l := range linksOf(true, gone, src) {
		if l.From == "src11111-bbbb" && !l.External {
			t.Fatalf("non-live peer resolved: %+v", l)
		}
	}
}

func namedLaunch(sec int, tool, id, name string) string {
	return assistantTool(sec, tool, "Agent", map[string]any{"subagent_type": "general-purpose", "description": "demo", "prompt": "p", "name": name}) +
		toolResult(sec+1, tool, "Async agent launched.", map[string]any{"agentId": id, "isAsync": true, "status": "async_launched"})
}

func TestSendByAgentNameResolves(t *testing.T) {
	// Session -> its agent by name, and agent -> sibling by name.
	subA := assistantTool(5, "m1", "SendMessage", map[string]any{"to": "demo-b", "message": "hi b"})
	v := sessionOf("sess1111-aaaa", namedLaunch(0, "t1", agA, "demo-a")+namedLaunch(1, "t2", agB, "demo-b")+sendMsg(3, "m0", "demo-a", "go"),
		map[string]string{agA: subA})
	ls := linksOf(true, v)
	if l := find(ls, "sess1111-aaaa", "", "sess1111-aaaa", agA, "message"); l == nil || l.External || l.ToLabel != "general-purpose a39a913" {
		t.Fatalf("session->agent by name %+v", ls)
	}
	if l := find(ls, "sess1111-aaaa", agA, "sess1111-aaaa", agB, "message"); l == nil || l.External {
		t.Fatalf("agent->sibling by name %+v", ls)
	}
}

func TestReusedAgentNameMeansNewestAndUnknownStaysExternal(t *testing.T) {
	const agC = "a22222222222222cc"
	v := sessionOf("sess1111-aaaa", namedLaunch(0, "t1", agA, "worker")+namedLaunch(10, "t2", agC, "worker")+
		sendMsg(20, "m0", "worker", "go")+sendMsg(21, "m1", "nobody", "go"), nil)
	ls := linksOf(true, v)
	if find(ls, "sess1111-aaaa", "", "sess1111-aaaa", agC, "message") == nil {
		t.Fatalf("reused name did not pick the newest launch %+v", ls)
	}
	if l := find(ls, "sess1111-aaaa", "", "nobody", "", "message"); l == nil || !l.External {
		t.Fatalf("unknown name should stay external %+v", ls)
	}
}

func TestPeerLineFromNamedSiblingResolvesAndSelfIsSkipped(t *testing.T) {
	subB := peerLine(7, "demo-a", false) + peerLine(8, "demo-b", false)
	v := sessionOf("sess1111-aaaa", namedLaunch(0, "t1", agA, "demo-a")+namedLaunch(1, "t2", agB, "demo-b"), map[string]string{agB: subB})
	ls := linksOf(true, v)
	if find(ls, "sess1111-aaaa", agA, "sess1111-aaaa", agB, "message") == nil {
		t.Fatalf("peer line from a named sibling %+v", ls)
	}
	if find(ls, "sess1111-aaaa", agB, "sess1111-aaaa", agB, "message") != nil {
		t.Fatalf("an agent's own name drew a self link %+v", ls)
	}
}
