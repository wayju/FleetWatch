// describe.go: the effective-settings report for the startup log and /health.
package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Profile is the effective runtimes and agents, for /health and the log.
func (r *Resolved) Profile() map[string]any {
	var rts, ags []string
	for _, s := range r.Runtimes {
		rts = append(rts, s.Type)
	}
	for _, s := range r.Agents {
		ags = append(ags, s.Type)
	}
	return map[string]any{"runtimes": rts, "agents": ags, "config": r.Path}
}

// Describe lists the effective settings and where each came from, for the
// startup log.
func (r *Resolved) Describe() []string {
	keys := make([]string, 0, len(r.Sources))
	for k := range r.Sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []string{"config: " + r.Path}
	for _, k := range keys {
		out = append(out, fmt.Sprintf("  %-22s %-8s %s", k, r.Sources[k], r.value(k)))
	}
	return out
}

func (r *Resolved) value(k string) string {
	switch k {
	case "host":
		return r.Host
	case "home":
		return r.Home
	case "poll":
		return r.Poll.String()
	case "window":
		return r.Window.String()
	case "recent":
		return r.Recent.String()
	case "rescan":
		return r.Rescan.String()
	case "show_text":
		return strconv.FormatBool(r.ShowText)
	case "port":
		return r.Port
	case "allowed_hosts":
		return strings.Join(r.AllowedHosts, ",")
	case "max_subscribers":
		return strconv.Itoa(r.MaxSubscribers)
	case "bind":
		return r.Bind
	case "hub":
		return r.hubDescribe()
	case "forges":
		return r.forgesDescribe()
	case "worktree_roots":
		return r.rootsDescribe()
	case "branch_pattern":
		return r.BranchPattern
	}
	if v, ok := r.sectionValue(k); ok {
		return v
	}
	return r.adapterValue(k)
}
