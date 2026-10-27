package fleettest

import (
	"fmt"
	"time"
)

// ParkedID is the shape Claude Code wrote for a reviewer that stopped to wait
// on its own CI watch (session 8acac073, task a0ee46c362f79143d).
const ParkedID = "a0ee46c362f79143d"

// WakeCall is a ScheduleWakeup call with its receipt.
func WakeCall(sec int, id string, in map[string]any) string {
	return AssistantTool(sec, id, "ScheduleWakeup", in) +
		ToolResult(sec+1, id, "scheduled", map[string]any{"clampedDelaySeconds": 1800, "wasClamped": false,
			"scheduledFor": T0.Add(time.Duration(sec)*time.Second + time.Duration(in["delaySeconds"].(int))*time.Second).UnixMilli()})
}

// BgBash is a background Bash launch with its receipt.
func BgBash(sec int, id, task string) string {
	return AssistantTool(sec, id, "Bash", map[string]any{"command": "sleep 99", "run_in_background": true}) +
		ToolResult(sec+1, id, "started", map[string]any{"backgroundTaskId": task})
}

// Monitor is a Monitor launch with its receipt.
func Monitor(sec int, id, task string) string {
	return AssistantTool(sec, id, "Monitor", map[string]any{"command": "tail -f x", "description": "d", "timeout_ms": 1000}) +
		ToolResult(sec+1, id, "started", map[string]any{"taskId": task, "persistent": false})
}

// Notification is a task notification naming a background task.
func Notification(sec int, task, tool string) string {
	body := fmt.Sprintf("<task-notification>\n<task-id>%s</task-id>\n<tool-use-id>%s</tool-use-id>\n<status>completed</status>\n</task-notification>", task, tool)
	return Line(map[string]any{"type": "user", "timestamp": TS(sec), "origin": map[string]any{"kind": "task-notification"},
		"message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": body}}}})
}

// AgentNote is a parked-agent task notification.
func AgentNote(sec int, summary string) string {
	body := fmt.Sprintf("<task-notification>\n<task-id>%s</task-id>\n<tool-use-id>toolu_01ABC</tool-use-id>\n<output-file>/tmp/claude/tasks/%s.output</output-file>\n<status>completed</status>\n<summary>%s</summary>\n</task-notification>\nThe result may be interim.", ParkedID, ParkedID, summary)
	return Line(map[string]any{"type": "user", "timestamp": TS(sec), "isMeta": true,
		"message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "text", "text": body}}}})
}

// ParkedSum is the summary Claude Code writes when an agent parks.
func ParkedSum(desc string) string {
	return fmt.Sprintf(`Agent "%s" is waiting on its own background work`, desc)
}

// LaunchParkable is a background agent launch that may later park.
func LaunchParkable(sec int) string {
	return AssistantTool(sec, "t1", "Agent", map[string]any{"subagent_type": "code-reviewer", "description": "Opus review", "prompt": "review", "run_in_background": true}) +
		ToolResult(sec+1, "t1", "Async agent launched. agentId: "+ParkedID, map[string]any{"isAsync": true, "status": "async_launched"})
}

// PeerLine is a peer line received from another session or agent.
func PeerLine(sec int, from string, handback bool) string {
	return Line(map[string]any{"type": "user", "isMeta": true, "timestamp": TS(sec),
		"origin":  map[string]any{"kind": "peer", "from": from, "name": "code-reviewer", "handback": handback},
		"message": map[string]any{"role": "user", "content": "<teammate-message>report body token=supersecretvalue123</teammate-message>"}})
}

// AsyncLaunch is an async agent launch with its receipt.
func AsyncLaunch(sec int, tool, id string) string {
	return AssistantTool(sec, tool, "Agent", map[string]any{"subagent_type": "code-reviewer", "description": "review", "prompt": "p"}) +
		ToolResult(sec+1, tool, "Async agent launched.", map[string]any{"agentId": id, "isAsync": true, "status": "async_launched"})
}

// SendMsg is a SendMessage call with its receipt.
func SendMsg(sec int, tool, to, msg string) string {
	return AssistantTool(sec, tool, "SendMessage", map[string]any{"to": to, "message": msg})
}
