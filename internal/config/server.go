// server.go: the server section (port, bind, allowed hosts, subscriber cap).
package config

import (
	"fmt"
	"strconv"
	"strings"
)

type serverSection struct {
	Port           *int     `json:"port"`
	AllowedHosts   []string `json:"allowed_hosts"`
	MaxSubscribers *int     `json:"max_subscribers"`
	Bind           *string  `json:"bind"`
	MemoryLimitMiB *int     `json:"memory_limit_mib"`
}

func (r *Resolved) serverFromFile(s *serverSection) error {
	if s == nil {
		return nil
	}
	if s.Port != nil {
		if *s.Port < 1 || *s.Port > 65535 {
			return fmt.Errorf("config %s: server.port %d is out of range", r.Path, *s.Port)
		}
		r.Port, r.Sources["port"] = strconv.Itoa(*s.Port), "file"
	}
	if s.AllowedHosts != nil {
		r.AllowedHosts, r.Sources["allowed_hosts"] = lowerAll(s.AllowedHosts), "file"
	}
	if s.MaxSubscribers != nil {
		if *s.MaxSubscribers < 1 {
			return fmt.Errorf("config %s: server.max_subscribers must be at least 1", r.Path)
		}
		r.MaxSubscribers, r.Sources["max_subscribers"] = *s.MaxSubscribers, "file"
	}
	if s.Bind != nil {
		r.Bind, r.Sources["bind"] = strings.TrimSpace(*s.Bind), "file"
	}
	if s.MemoryLimitMiB != nil {
		if *s.MemoryLimitMiB < 0 {
			return fmt.Errorf("config %s: server.memory_limit_mib must be 0 (no limit set) or a size in MiB", r.Path)
		}
		r.MemoryLimitMiB, r.Sources["memory_limit_mib"] = *s.MemoryLimitMiB, "file"
	}
	return nil
}

// lowerAll trims and lower-cases, dropping empties.
func lowerAll(in []string) []string {
	var out []string
	for _, p := range in {
		if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
			out = append(out, p)
		}
	}
	return out
}
