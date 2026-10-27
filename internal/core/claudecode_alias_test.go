package core

import "github.com/wayju/fleetwatch/internal/agent/claudecode"

// The transcript fold lives in internal/agent/claudecode; the end-to-end tests
// here build transcripts through these short names.
type (
	Transcript = claudecode.Transcript
	ToolCall   = claudecode.ToolCall
)

var (
	newTranscript = claudecode.NewTranscript
	digestOf      = claudecode.DigestOf
	agentsOf      = claudecode.AgentsOf
	mainLinks     = claudecode.MainLinks
)

const (
	parkStale = claudecode.ParkStale
	maxAgents = claudecode.MaxAgents
)
