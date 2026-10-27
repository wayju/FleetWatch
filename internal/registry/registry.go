// Package registry is the one place adapters are known by name. A runtime
// adapter (where agent processes live) or an agent adapter (how a session is
// read) registers a decoder for its config entry; config resolves a "type"
// through here and builds what the decoder returned.
//
// Registration is explicit: an adapter exists in a binary only if something
// compiled into it calls Register. The built-in adapters register from
// internal/config; an optional adapter registers from its own file under
// cmd/fleetwatch, so deleting that file (and the adapter's package) removes it
// from the binary without touching any other code.
package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/models"
)

// Resolver is what a decoder may ask of the configuration around it: paths
// resolved against the fleet home, the precedence of environment over file over
// default, and where each value came from.
type Resolver interface {
	// Pick applies the precedence for one setting (environment variable named
	// envName if set, else fileVal, else def), expands a leading ~ and records
	// the source under key. An empty envName means the setting has no variable.
	Pick(key, envName, fileVal, def string) string
	// Record notes where a setting came from ("default", "file" or "env").
	Record(key, source string)
	// Expand resolves a leading ~ against the fleet home.
	Expand(path string) string
	// Home is the fleet home directory.
	Home() string
	// UID is the numeric user id the service runs as.
	UID() int
	// Getenv reads one environment variable.
	Getenv(name string) string
	// ReadFile reads a file the config names (a tmux rules file, say).
	ReadFile(path string) ([]byte, error)
}

// BuildCtx carries the fleet-wide settings an adapter may need when it is
// constructed.
type BuildCtx struct {
	Poll       time.Duration
	Window     time.Duration
	Rescan     time.Duration
	PricesFile string
	Models     *models.Table
	// ExtraDirs lists agent config dirs that the built runtimes know about.
	ExtraDirs func() []host.ConfigDir
	Logf      func(format string, args ...any)
}

// RuntimeSettings is one configured runtime entry after decoding: its resolved
// paths, ready to construct the runtime.
type RuntimeSettings interface {
	// Build constructs the runtime. The runtime may also implement the optional
	// interfaces in internal/host (Observer, Watcher, ConfigDirSource,
	// ActivitySource, ContextSource); the core discovers them by assertion.
	Build(BuildCtx) host.Runtime
	// Value renders one resolved setting by its key within the entry
	// ("pool", "proc_root"), for the startup report.
	Value(key string) (string, bool)
}

// AgentSettings is one configured agent entry after decoding.
type AgentSettings interface {
	Build(BuildCtx) agent.Agent
	Value(key string) (string, bool)
}

// Decoder turns the raw JSON of one config entry into settings. raw has been
// checked to carry a "type"; the decoder must refuse unknown keys (Strict).
// idx is the entry's position in its list, for source keys such as
// "runtimes[1].proc_root".
type (
	RuntimeDecoder func(r Resolver, raw json.RawMessage, idx int) (RuntimeSettings, error)
	AgentDecoder   func(r Resolver, raw json.RawMessage, idx int) (AgentSettings, error)
)

var (
	mu       sync.RWMutex
	runtimes = map[string]RuntimeDecoder{}
	agents   = map[string]AgentDecoder{}
)

// RegisterRuntime makes a runtime adapter available under name. It panics on a
// duplicate: two registrations of one name is a programming error.
func RegisterRuntime(name string, d RuntimeDecoder) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := runtimes[name]; dup {
		panic("registry: runtime " + name + " registered twice")
	}
	runtimes[name] = d
}

// RegisterAgent makes an agent adapter available under name.
func RegisterAgent(name string, d AgentDecoder) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := agents[name]; dup {
		panic("registry: agent " + name + " registered twice")
	}
	agents[name] = d
}

// Runtime returns the decoder registered for name.
func Runtime(name string) (RuntimeDecoder, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := runtimes[name]
	return d, ok
}

// Agent returns the decoder registered for name.
func Agent(name string) (AgentDecoder, bool) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := agents[name]
	return d, ok
}

// RuntimeNames lists the registered runtimes, sorted, as "a, b, c".
func RuntimeNames() string { mu.RLock(); defer mu.RUnlock(); return names(runtimes) }

// AgentNames lists the registered agents, sorted, as "a, b, c".
func AgentNames() string { mu.RLock(); defer mu.RUnlock(); return names(agents) }

func names[T any](m map[string]T) string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// Strict decodes raw into v, refusing unknown keys and trailing data. A
// misspelt key that silently does nothing is the failure this prevents.
func Strict(raw []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("unexpected data after the JSON document")
	}
	return nil
}

// TypeOf reads an entry's "type" before the adapter's own strict decode.
func TypeOf(raw json.RawMessage) (string, error) {
	var t struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return "", err
	}
	if t.Type == "" {
		return "", fmt.Errorf("missing \"type\"")
	}
	return t.Type, nil
}
