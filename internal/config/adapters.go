package config

import (
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wayju/fleetwatch/internal/agent"
	"github.com/wayju/fleetwatch/internal/host"
	"github.com/wayju/fleetwatch/internal/registry"
)

// Builtin adapters register here; an optional adapter registers from its own
// file under cmd/fleetwatch. See internal/registry.
func init() {
	registry.RegisterRuntime("proc", asRuntime(decodeProc))
	registry.RegisterRuntime("tmux", asRuntime(decodeTmux))
	registry.RegisterRuntime("herdr", asRuntime(decodeHerdr))
	registry.RegisterRuntime("hookfile", asRuntime(decodeHookfile))
	registry.RegisterAgent("claudecode", func(r registry.Resolver, raw json.RawMessage, i int) (registry.AgentSettings, error) {
		s, err := decodeClaudecode(r, raw, i)
		return s, err
	})
}

func asRuntime(f func(registry.Resolver, json.RawMessage, int) (RuntimeSpec, error)) registry.RuntimeDecoder {
	return func(r registry.Resolver, raw json.RawMessage, i int) (registry.RuntimeSettings, error) {
		s, err := f(r, raw, i)
		return s, err
	}
}

// resolver lets a decoder in another package see the configuration around its
// entry without importing this one.
type resolver struct {
	r   *Resolved
	env Env
}

func (v resolver) Pick(key, envName, fileVal, def string) string {
	return v.r.pick(v.env, key, envName, fileVal, def)
}
func (v resolver) Record(key, source string)         { v.r.Sources[key] = source }
func (v resolver) Expand(p string) string            { return v.r.expand(p) }
func (v resolver) Home() string                      { return v.r.Home }
func (v resolver) UID() int                          { return v.r.uid }
func (v resolver) Getenv(name string) string         { return v.env.Get(name) }
func (v resolver) ReadFile(p string) ([]byte, error) { return v.env.ReadFile(p) }

// adapters resolves the runtimes and agents: the file's lists, or the
// zero-config default (proc and claudecode) when no file was read.
func (r *Resolved) adapters(env Env, doc *fileDoc) error {
	res := resolver{r, env}
	if doc == nil || (doc.Runtimes == nil && doc.Agents == nil) {
		r.Runtimes = []RuntimeEntry{r.mustRuntime(res, "proc")}
		if env.LookPath != nil {
			if _, err := env.LookPath("tmux"); err == nil {
				r.Runtimes = append(r.Runtimes, r.mustRuntime(res, "tmux"))
			}
		}
		r.Agents = []AgentEntry{r.mustAgent(res, "claudecode")}
		r.Sources["runtimes"], r.Sources["agents"] = "default", "default"
		return nil
	}
	if len(doc.Runtimes) == 0 || len(doc.Agents) == 0 {
		return fmt.Errorf("config %s: runtimes and agents must each list at least one adapter", r.Path)
	}
	for i, raw := range doc.Runtimes {
		typ, err := registry.TypeOf(raw)
		if err != nil {
			return fmt.Errorf("config %s: runtimes[%d]: %v", r.Path, i, err)
		}
		dec, ok := registry.Runtime(typ)
		if !ok {
			return fmt.Errorf("config %s: runtimes[%d]: unknown type %q (known: %s)", r.Path, i, typ, registry.RuntimeNames())
		}
		spec, err := dec(res, raw, i)
		if err != nil {
			return fmt.Errorf("config %s: runtimes[%d] (%s): %v", r.Path, i, typ, err)
		}
		r.Runtimes = append(r.Runtimes, RuntimeEntry{Type: typ, Settings: spec})
	}
	for i, raw := range doc.Agents {
		typ, err := registry.TypeOf(raw)
		if err != nil {
			return fmt.Errorf("config %s: agents[%d]: %v", r.Path, i, err)
		}
		dec, ok := registry.Agent(typ)
		if !ok {
			return fmt.Errorf("config %s: agents[%d]: unknown type %q (known: %s)", r.Path, i, typ, registry.AgentNames())
		}
		spec, err := dec(res, raw, i)
		if err != nil {
			return fmt.Errorf("config %s: agents[%d] (%s): %v", r.Path, i, typ, err)
		}
		r.Agents = append(r.Agents, AgentEntry{Type: typ, Settings: spec})
	}
	r.Sources["runtimes"], r.Sources["agents"] = "file", "file"
	return nil
}

// mustRuntime decodes a built-in runtime with all-default settings.
func (r *Resolved) mustRuntime(res resolver, typ string) RuntimeEntry {
	dec, _ := registry.Runtime(typ)
	s, err := dec(res, json.RawMessage(`{"type":"`+typ+`"}`), len(r.Runtimes))
	if err != nil {
		panic(err)
	}
	return RuntimeEntry{Type: typ, Settings: s}
}

func (r *Resolved) mustAgent(res resolver, typ string) AgentEntry {
	dec, _ := registry.Agent(typ)
	s, err := dec(res, json.RawMessage(`{"type":"`+typ+`"}`), len(r.Agents))
	if err != nil {
		panic(err)
	}
	return AgentEntry{Type: typ, Settings: s}
}

// expand resolves a leading ~ against the resolved home, not the process's.
func (r *Resolved) expand(p string) string {
	switch {
	case p == "~":
		return r.Home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(r.Home, p[2:])
	}
	return p
}

// pick applies the precedence for one adapter path: environment, then the
// file, then the default; the source is recorded under key.
func (r *Resolved) pick(env Env, key, envName, fileVal, def string) string {
	switch {
	case envName != "" && env.Get(envName) != "":
		r.Sources[key] = "env"
		return r.expand(env.Get(envName))
	case fileVal != "":
		r.Sources[key] = "file"
		return r.expand(fileVal)
	}
	r.Sources[key] = "default"
	return r.expand(def)
}

type procDoc struct {
	Type     string `json:"type"`
	ProcRoot string `json:"proc_root"`
}

func decodeProc(res registry.Resolver, raw json.RawMessage, i int) (RuntimeSpec, error) {
	var d procDoc
	if err := registry.Strict(raw, &d); err != nil {
		return RuntimeSpec{}, err
	}
	k := "runtimes[" + strconv.Itoa(i) + "]."
	return RuntimeSpec{Type: "proc", ProcRoot: res.Pick(k+"proc_root", "FLEET_PROC_ROOT", d.ProcRoot, "/proc")}, nil
}

type claudecodeDoc struct {
	Type       string   `json:"type"`
	ConfigDirs []string `json:"config_dirs"`
}

func decodeClaudecode(res registry.Resolver, raw json.RawMessage, i int) (AgentSpec, error) {
	var d claudecodeDoc
	if err := registry.Strict(raw, &d); err != nil {
		return AgentSpec{}, err
	}
	k := "agents[" + strconv.Itoa(i) + "].config_dirs"
	s := AgentSpec{Type: "claudecode"}
	if len(d.ConfigDirs) == 0 {
		res.Record(k, "default")
		s.ConfigDirs = []string{res.Expand("~/.claude")}
		return s, nil
	}
	res.Record(k, "file")
	for _, p := range d.ConfigDirs {
		s.ConfigDirs = append(s.ConfigDirs, res.Expand(p))
	}
	return s, nil
}

// adapterValue renders an adapter setting for Describe: the key names the entry
// ("runtimes[1].pool") and the entry's own settings answer for the rest.
func (r *Resolved) adapterValue(key string) string {
	for i, e := range r.Runtimes {
		if rest, ok := strings.CutPrefix(key, "runtimes["+strconv.Itoa(i)+"]."); ok {
			if v, ok := e.Settings.Value(rest); ok {
				return v
			}
		}
	}
	for i, e := range r.Agents {
		if rest, ok := strings.CutPrefix(key, "agents["+strconv.Itoa(i)+"]."); ok {
			if v, ok := e.Settings.Value(rest); ok {
				return v
			}
		}
	}
	return ""
}

// Build constructs the configured runtimes and agents. The agents' extra
// transcript roots come from whichever runtimes know of config dirs.
func (r *Resolved) Build() ([]host.Runtime, []agent.Agent) {
	ctx := registry.BuildCtx{Poll: r.Poll, Window: r.Window, Rescan: r.Rescan, PricesFile: r.PricesFile, Models: r.Models, Logf: log.Printf}
	var rts []host.Runtime
	for _, e := range r.Runtimes {
		if rt := e.Settings.Build(ctx); rt != nil {
			rts = append(rts, rt)
		}
	}
	ctx.ExtraDirs = func() []host.ConfigDir {
		var out []host.ConfigDir
		for _, rt := range rts {
			if d, ok := rt.(host.ConfigDirSource); ok {
				out = append(out, d.ConfigDirs()...)
			}
		}
		return out
	}
	var ags []agent.Agent
	for _, e := range r.Agents {
		ags = append(ags, e.Settings.Build(ctx))
	}
	return rts, ags
}

func joinComma(s []string) string { return strings.Join(s, ",") }
