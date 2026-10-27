// Package fleettest holds the fixture builders the fleet's tests share: JSONL
// transcript lines and fake /proc entries. It is imported from _test.go files
// only, so it never reaches the binary.
package fleettest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// T0 is the fixture clock: the board's windows are measured from it.
var T0 = time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)

// TS is the RFC 3339 time sec seconds after T0.
func TS(sec int) string { return T0.Add(time.Duration(sec) * time.Second).Format(time.RFC3339Nano) }

// Line is one JSONL line.
func Line(m map[string]any) string {
	b, _ := json.Marshal(m)
	return string(b) + "\n"
}

// UserText is a real user prompt.
func UserText(sec int, text string) string {
	return Line(map[string]any{"type": "user", "timestamp": TS(sec), "cwd": "/repo", "gitBranch": "feature/42-fleet", "version": "2.1.3",
		"message": map[string]any{"role": "user", "content": text}})
}

// AssistantTool is an assistant line calling one tool.
func AssistantTool(sec int, id, name string, input map[string]any) string {
	return Line(map[string]any{"type": "assistant", "timestamp": TS(sec), "message": map[string]any{
		"role": "assistant", "model": "claude-sonnet-5-5",
		"usage":   map[string]any{"input_tokens": 10, "cache_read_input_tokens": 49990, "output_tokens": 0},
		"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}})
}

// ToolResult is the user line returning a tool's result.
func ToolResult(sec int, id, text string, extra map[string]any) string {
	m := map[string]any{"type": "user", "timestamp": TS(sec), "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": text}}}}
	if extra != nil {
		m["toolUseResult"] = extra
	}
	return Line(m)
}

// AssistantEnd is an assistant line that ends the turn.
func AssistantEnd(sec int, text string) string {
	return Line(map[string]any{"type": "assistant", "timestamp": TS(sec), "message": map[string]any{
		"role": "assistant", "model": "claude-sonnet-5-5", "stop_reason": "end_turn",
		"content": []any{map[string]any{"type": "text", "text": text}}}})
}

// Write creates path with body and stamps it with fixture time.
func Write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Fixture time, not wall time: the board's windows are measured from T0.
	os.Chtimes(path, T0, T0)
}

// FakeProc writes the /proc entries (stat, cmdline, cgroup) of one process.
func FakeProc(t *testing.T, root string, pid, ppid int, comm, cmdline, cgroup string, startTicks int64) {
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = "S"
	fields[1] = fmt.Sprint(ppid)
	fields[19] = fmt.Sprint(startTicks)
	Write(t, filepath.Join(root, fmt.Sprint(pid), "stat"), fmt.Sprintf("%d (%s) %s", pid, comm, strings.Join(fields, " ")))
	Write(t, filepath.Join(root, fmt.Sprint(pid), "cmdline"), strings.ReplaceAll(cmdline, " ", "\x00")+"\x00")
	Write(t, filepath.Join(root, fmt.Sprint(pid), "cgroup"), cgroup)
}
