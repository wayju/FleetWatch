package claudecode

import (
	"testing"
	"time"
)

// subAt is ag1's own transcript with its last line at sec: a Bash call still
// running, or an end_turn when ended is true.
func subAt(sec int, ended bool) map[string]*Transcript {
	s := NewTranscript("ag1")
	if ended {
		applyAll(s, assistantEnd(sec, "done"))
	} else {
		applyAll(s, assistantTool(sec, "b1", "Bash", map[string]any{"command": "sleep 20"}))
	}
	return map[string]*Transcript{"ag1": s}
}

func completedNote(sec int, id string) string {
	return line(map[string]any{"type": "user", "timestamp": ts(sec), "message": map[string]any{"role": "user",
		"content": "<task-notification><task-id>" + id + "</task-id><status>completed</status></task-notification>"}})
}

func TestWokenAgentRunsAgain(t *testing.T) {
	ends := map[string]string{"handback": peerLine(30, "ag1", true), "notification": completedNote(30, "ag1")}
	for name, end := range ends {
		tr := NewTranscript("s1")
		applyAll(tr, asyncLaunch(0, "t1", "ag1")+end+sendMsg(35, "t2", "ag1", "go on"))
		av := AgentsOf(t0.Add(50*time.Second), nil, tr, subAt(40, false))
		if len(av) != 1 || av[0].Status != "running" || av[0].EndedAt != 0 {
			t.Fatalf("%s then wake: %+v", name, av)
		}
	}
}

func TestWokenAgentEndsAtItsNextEndTurn(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+peerLine(30, "ag1", true))
	av := AgentsOf(t0.Add(70*time.Second), nil, tr, subAt(60, true))
	if len(av) != 1 || av[0].Status != "done" || av[0].EndedAt != t0.Add(60*time.Second).Unix() {
		t.Fatalf("%+v", av)
	}
}

func TestFinishedAgentWithinSlackStaysDone(t *testing.T) {
	tr := NewTranscript("s1")
	applyAll(tr, asyncLaunch(0, "t1", "ag1")+peerLine(30, "ag1", true))
	av := AgentsOf(t0.Add(40*time.Second), nil, tr, subAt(31, false))
	if len(av) != 1 || av[0].Status != "done" {
		t.Fatalf("a line just after the hand-back is not a wake: %+v", av)
	}
}
