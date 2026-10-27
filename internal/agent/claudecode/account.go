package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wayju/fleetwatch/internal/board"
	"github.com/wayju/fleetwatch/internal/fsafe"
)

// sessionFileInfo reads <config>/sessions/<pid>.json, which recent Claude Code
// versions write for each running process.
// It also returns the peer name, which SendMessage records as the recipient
// when one session addresses another by name.
func sessionFileInfo(configDir string, pid int) (id, name string) {
	b, err := fsafe.ReadSmall(filepath.Join(configDir, "sessions", strconv.Itoa(pid)+".json"), 64<<10)
	if err != nil {
		return "", ""
	}
	var s struct {
		SessionID string `json:"sessionId"`
		Name      string `json:"name"`
	}
	_ = json.Unmarshal(b, &s)
	return s.SessionID, s.Name
}

func isClaude(comm, cmdline string) bool {
	if comm == "claude" {
		return true
	}
	if strings.HasPrefix(comm, "node") {
		return strings.Contains(cmdline, "@anthropic-ai/claude-code") || strings.Contains(cmdline, "/bin/claude")
	}
	return false
}

// accountCache reads oauthAccount out of a slot's .claude.json, re-reading
// only when the file changes. The file is large and holds tokens and MCP
// config; nothing but the three identity fields is kept.
type accountCache struct {
	mu sync.Mutex
	m  map[string]accountEntry
}

type accountEntry struct {
	mod  time.Time
	size int64
	acc  board.Account
}

func (c *accountCache) get(path string) board.Account {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() {
		return board.Account{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]accountEntry{}
	}
	if e, ok := c.m[path]; ok && e.mod.Equal(st.ModTime()) && e.size == st.Size() {
		return e.acc
	}
	b, err := fsafe.ReadSmall(path, 32<<20)
	if err != nil {
		return board.Account{}
	}
	var doc struct {
		OAuth *struct {
			Email string `json:"emailAddress"`
			Name  string `json:"displayName"`
			Org   string `json:"organizationName"`
		} `json:"oauthAccount"`
	}
	var acc board.Account
	if json.Unmarshal(b, &doc) == nil && doc.OAuth != nil {
		acc = board.Account{Email: doc.OAuth.Email, Name: doc.OAuth.Name, Org: doc.OAuth.Org}
	}
	c.m[path] = accountEntry{mod: st.ModTime(), size: st.Size(), acc: acc}
	return acc
}
