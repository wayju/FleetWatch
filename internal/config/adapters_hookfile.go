package config

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/wayju/fleetwatch/internal/registry"
)

// defaultHookfileFresh is how long a state file's write stays authoritative.
// A waiting session is demoted to idle after 30 minutes anyway; an hour keeps a
// long approval prompt precise and still retires a crashed session's file.
const defaultHookfileFresh = time.Hour

type hookfileDoc struct {
	Type  string `json:"type"`
	Dir   string `json:"dir"`
	Fresh string `json:"fresh"`
}

// decodeHookfile resolves the state-file directory: FLEET_HOOKFILE_DIR, then
// the file, then $XDG_STATE_HOME/fleet/sessions, then ~/.local/state/fleet/sessions
// under the fleet home (see ADR-010).
func decodeHookfile(res registry.Resolver, raw json.RawMessage, i int) (RuntimeSpec, error) {
	var d hookfileDoc
	if err := registry.Strict(raw, &d); err != nil {
		return RuntimeSpec{}, err
	}
	k := "runtimes[" + strconv.Itoa(i) + "]."
	def := filepath.Join(res.Home(), ".local", "state", "fleet", "sessions")
	if x := res.Getenv("XDG_STATE_HOME"); x != "" {
		def = filepath.Join(x, "fleet", "sessions")
	}
	s := RuntimeSpec{Type: "hookfile", HookfileFresh: defaultHookfileFresh}
	s.HookfileDir = res.Pick(k+"dir", "FLEET_HOOKFILE_DIR", d.Dir, def)
	res.Record(k+"fresh", "default")
	if d.Fresh != "" {
		v, err := time.ParseDuration(d.Fresh)
		if err != nil || v <= 0 {
			return RuntimeSpec{}, fmt.Errorf("fresh must be a positive duration like \"1h\", got %q", d.Fresh)
		}
		s.HookfileFresh = v
		res.Record(k+"fresh", "file")
	}
	return s, nil
}
