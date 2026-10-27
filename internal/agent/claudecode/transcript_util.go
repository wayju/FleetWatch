package claudecode

// Tool-call input helpers for the transcript reader.

import (
	"github.com/wayju/fleetwatch/internal/textsafe"
	"strings"
)

// toolTarget is the short "what" of a tool call: the file, command, pattern or
// URL it acts on, redacted and clipped for display.
func toolTarget(name string, in map[string]any) string {
	for _, k := range []string{"file_path", "notebook_path", "command", "pattern", "url", "query", "skill", "description", "path", "to", "prompt"} {
		if s := str(in, k); s != "" {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[:i] + " …"
			}
			return textsafe.Clip(textsafe.Redact(s), 90)
		}
	}
	return ""
}

func str(m map[string]any, k string) string {
	if s, ok := m[k].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func firstStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if s := str(m, k); s != "" {
			return s
		}
	}
	return ""
}
