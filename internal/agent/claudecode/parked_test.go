package claudecode

import (
	"testing"
)

func TestParkedNotificationParksAgent(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "go")+launchParkable(1)+agentNote(60, parkedSum("Opus review")))
	if a := tr.Agents[0]; a.Status != "parked" || !a.EndedAt.IsZero() {
		t.Fatalf("status %q ended %v", a.Status, a.EndedAt)
	}
}

func TestLaterDifferentSummaryEndsParked(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "go")+launchParkable(1)+agentNote(60, parkedSum("Opus review"))+agentNote(120, `Agent "Opus review" completed`))
	if a := tr.Agents[0]; a.Status != "done" || a.EndedAt.IsZero() {
		t.Fatalf("status %q", a.Status)
	}
	// The same summary again keeps it parked.
	tr = NewTranscript("s2")
	applyAll(tr, userText(0, "go")+launchParkable(1)+agentNote(60, parkedSum("Opus review"))+agentNote(120, parkedSum("Opus review")))
	if tr.Agents[0].Status != "parked" {
		t.Fatalf("repeat notification: %q", tr.Agents[0].Status)
	}
}

func TestHandbackEndsParked(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "go")+launchParkable(1)+agentNote(60, parkedSum("x"))+peerLine(90, parkedID, true))
	if tr.Agents[0].Status != "done" {
		t.Fatalf("status %q", tr.Agents[0].Status)
	}
}

func TestParkedAgentIsConditionWake(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, userText(0, "go")+launchParkable(1)+agentNote(60, parkedSum("x"))+assistantEnd(61, "waiting"))
	w := NextWake(tr, "waiting", 0, 1, nil, ident)
	if w == nil || w.Kind != "condition" || w.Reason != "1 agent waiting on background work" {
		t.Fatalf("got %+v", w)
	}
	if w = NextWake(tr, "idle", 1, 2, nil, ident); w == nil || w.Reason != "1 background task, 2 agents waiting on background work" {
		t.Fatalf("merged: %+v", w)
	}
	if w = NextWake(tr, "waiting", 0, 0, nil, ident); w != nil {
		t.Fatalf("nothing parked: %+v", w)
	}
}
