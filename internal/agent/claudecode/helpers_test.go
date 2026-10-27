package claudecode

import (
	"strings"
	"testing"
	"time"

	"github.com/wayju/fleetwatch/internal/fleettest"
)

var (
	t0            = fleettest.T0
	ts            = fleettest.TS
	line          = fleettest.Line
	userText      = fleettest.UserText
	assistantTool = fleettest.AssistantTool
	toolResult    = fleettest.ToolResult
	assistantEnd  = fleettest.AssistantEnd
	write         = fleettest.Write

	wakeCall       = fleettest.WakeCall
	bgBash         = fleettest.BgBash
	monitor        = fleettest.Monitor
	notification   = fleettest.Notification
	agentNote      = fleettest.AgentNote
	parkedSum      = fleettest.ParkedSum
	launchParkable = fleettest.LaunchParkable
	peerLine       = fleettest.PeerLine
	asyncLaunch    = fleettest.AsyncLaunch
	sendMsg        = fleettest.SendMsg
)

const parkedID = fleettest.ParkedID

func applyAll(t *Transcript, s string) {
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		t.Apply([]byte(l))
	}
}

var _ = time.Second
var _ = testing.Short
