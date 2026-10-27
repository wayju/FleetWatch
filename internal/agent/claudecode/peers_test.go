package claudecode

import (
	"strings"
	"testing"
	"time"
)

func kinds(evs []agentEvent) string {
	var k []string
	for _, e := range evs {
		k = append(k, e.Kind)
	}
	return strings.Join(k, ",")
}

func TestToolDescriptionCapturedRedactedAndClipped(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, assistantTool(0, "t1", "Bash", map[string]any{"command": "go test", "description": "Running tests token=supersecretvalue123 " + strings.Repeat("x", 300)}))
	d := tr.Tools[0].Description
	if !strings.HasPrefix(d, "Running tests") || strings.Contains(d, "supersecretvalue123") || len([]rune(d)) > 140 {
		t.Fatalf("description %q", d)
	}
}

func TestPeerHandbackAttributedAndFinishesRunningAgent(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+asyncLaunch(10, "t2", "ag2")+peerLine(30, "ag1", true))
	a1, a2 := tr.Agents[0], tr.Agents[1]
	if a1.Status != "done" || !a1.EndedAt.Equal(t0.Add(30*time.Second)) || !a1.HandedBackAt.Equal(t0.Add(30*time.Second)) {
		t.Fatalf("handback must finish ag1: %+v", a1)
	}
	if a2.Status != "running" || !a2.HandedBackAt.IsZero() {
		t.Fatalf("ag2 untouched: %+v", a2)
	}
	if got := kinds(a1.events); got != "launch,handback" {
		t.Fatalf("events %s", got)
	}
	if tr.FirstPrompt != "" || tr.LastKind == "prompt" {
		t.Fatalf("peer line is not a prompt: %q", tr.FirstPrompt)
	}
	applyAll(tr, peerLine(40, "nobody", true)) // unknown sender is ignored, not a panic
}

func TestPeerMessageIsFromEventAndDoesNotFinish(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+peerLine(5, "ag1", false))
	a := tr.Agents[0]
	if a.Status != "running" || !a.HandedBackAt.IsZero() || kinds(a.events) != "launch,from" {
		t.Fatalf("%+v %s", a, kinds(a.events))
	}
}

func TestSendMessageToAgentIsToEventAndLaunchUsesReceiptTime(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+assistantTool(8, "t2", "SendMessage", map[string]any{"to": "ag1", "message": "go on"}))
	a := tr.Agents[0]
	if kinds(a.events) != "launch,to" || !a.events[0].At.Equal(t0.Add(1*time.Second)) || !a.events[1].At.Equal(t0.Add(8*time.Second)) {
		t.Fatalf("events %+v", a.events)
	}
}

func TestNotificationEmitsDoneOrErrorEvent(t *testing.T) {
	tr := NewTranscript("s1")
	note := func(sec int, id, status string) string {
		return line(map[string]any{"type": "user", "timestamp": ts(sec), "message": map[string]any{"role": "user",
			"content": "<task-notification><task-id>" + id + "</task-id><status>" + status + "</status></task-notification>"}})
	}
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+asyncLaunch(2, "t2", "ag2")+note(9, "ag1", "completed")+note(9, "ag2", "failed"))
	if tr.Agents[0].Status != "done" || kinds(tr.Agents[0].events) != "launch,done" {
		t.Fatalf("ag1 %+v", tr.Agents[0])
	}
	if tr.Agents[1].Status != "error" || kinds(tr.Agents[1].events) != "launch,error" {
		t.Fatalf("ag2 %+v", tr.Agents[1])
	}
}

func TestAgentEventsCappedAtEight(t *testing.T) {
	tr := NewTranscript("s1")
	s := asyncLaunch(0, "t1", "ag1")
	for i := 0; i < 12; i++ {
		s += peerLine(10+i, "ag1", false)
	}
	applyAll(tr, s)
	av := AgentsOf(t0.Add(time.Minute), nil, tr, nil)
	if len(av) != 1 || len(av[0].Events) != 8 || av[0].Events[7].At != t0.Add(21*time.Second).Unix() || av[0].Events[0].Kind != "from" {
		t.Fatalf("%+v", av)
	}
}
