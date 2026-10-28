// Package config resolves the fleet's configuration: built-in defaults, then
// an optional strict-JSON file, then environment variables, and builds the
// runtimes and agents it names.
//
// The file is decoded strictly (unknown keys are an error, there are no
// comments), because a misspelt key that silently does nothing is the
// "config written is not config in effect" failure. It carries no secrets:
// FLEET_TOKEN and FLEET_RECAP_KEY stay in the environment and no struct here has
// a field for them, so a file that names one fails to load. A forge names its
// token by reference (token_env or token_file); the value is read at startup and
// is never logged.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/wayju/fleetwatch/internal/models"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wayju/fleetwatch/internal/forge"
	"github.com/wayju/fleetwatch/internal/registry"
)

// Resolved is the effective configuration.
type Resolved struct {
	Host           string
	Home           string
	Poll           time.Duration
	Window         time.Duration
	Recent         time.Duration
	Rescan         time.Duration
	ShowText       bool
	Port           string
	AllowedHosts   []string
	MaxSubscribers int
	PricesFile     string        // cost.prices / FLEET_PRICES_FILE; empty prices nothing
	Models         *models.Table // the models section; never nil
	RecapURL       string        // recap needs url, model and FLEET_RECAP_KEY, or it stays off
	RecapModel     string
	RecapEvery     time.Duration
	Runtimes       []RuntimeEntry
	Agents         []AgentEntry
	Forges         []forge.Spec   // empty: no issue titles, no merged state
	WorktreeRoots  []WorktreeRoot // where repos are scanned for worktrees
	BranchPattern  string         // one capture group: the issue number
	Bind           string         // listen address host; default 127.0.0.1, "0.0.0.0" for all
	Hub            *HubSpec       // nil: single-host mode (see ADR-008)
	MemoryLimitMiB int            // server.memory_limit_mib; 0 leaves the Go runtime default
	UIEnabled      bool           // ui.enabled: serve the page and the session route; default true
	UIThemeFile    string         // ui.theme_file: served as theme.css in place of the built-in theme
	Warnings       []string       // startup lines for settings that are legal but weaken security

	Path    string            // the file read, or "default"
	Sources map[string]string // key -> default | file | env
	uid     int
}

type fileDoc struct {
	Version  int               `json:"version"`
	Host     *string           `json:"host"`
	Home     *string           `json:"home"`
	Poll     *string           `json:"poll"`
	Window   *string           `json:"window"`
	Recent   *string           `json:"recent"`
	Rescan   *string           `json:"rescan"`
	ShowText *bool             `json:"show_text"`
	Server   *serverSection    `json:"server"`
	Runtimes []json.RawMessage `json:"runtimes"`
	Agents   []json.RawMessage `json:"agents"`
	Forges   []json.RawMessage `json:"forges"`

	Worktrees json.RawMessage `json:"worktrees"`
	Issues    json.RawMessage `json:"issues"`

	Cost   json.RawMessage `json:"cost"`
	Models json.RawMessage `json:"models"`
	Recap  json.RawMessage `json:"recap"`
	Hub    json.RawMessage `json:"hub"`
	UI     json.RawMessage `json:"ui"`
}

// strict decodes raw into v, refusing unknown keys and trailing data.
func strict(raw []byte, v any) error { return registry.Strict(raw, v) }

// Resolve reads the configuration. FLEET_CONFIG names the file and it must
// load; without it ~/.config/fleet/config.json (or $XDG_CONFIG_HOME/fleet/)
// is tried, and no file at all means the zero-config default. A file that is
// present but broken is an error, never a fallback to the default.
func Resolve(env Env) (*Resolved, error) {
	r := &Resolved{Path: "default", Sources: map[string]string{}, uid: env.UID}
	doc, err := r.load(env)
	if err != nil {
		return nil, err
	}
	if err := r.top(env, doc); err != nil {
		return nil, err
	}
	if err := r.adapters(env, doc); err != nil {
		return nil, err
	}
	if err := r.forgesAndRepos(env, doc); err != nil {
		return nil, err
	}
	if err := r.hubFromFile(env, doc); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Resolved) load(env Env) (*fileDoc, error) {
	path, required := env.Get("FLEET_CONFIG"), true
	if path == "" {
		required = false
		base := env.Get("XDG_CONFIG_HOME")
		if base == "" {
			home, err := env.UserHomeDir()
			if err != nil {
				return nil, nil
			}
			base = filepath.Join(home, ".config")
		}
		path = filepath.Join(base, "fleet", "config.json")
	}
	raw, err := env.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	var doc fileDoc
	if err := strict(raw, &doc); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	if doc.Version != 1 {
		return nil, fmt.Errorf("config %s: version must be 1, got %d", path, doc.Version)
	}
	r.Path = path
	return &doc, nil
}

func present(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null"
}
