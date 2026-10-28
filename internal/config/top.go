// top.go: the top-level scalar settings, from file then environment.
package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (r *Resolved) top(env Env, doc *fileDoc) error {
	if doc == nil {
		doc = &fileDoc{}
	}
	hostname, _ := env.Hostname()
	userHome, _ := env.UserHomeDir()
	r.Host, r.Home = hostname, userHome
	r.Sources["host"], r.Sources["home"] = "default", "default"
	r.Poll, r.Window, r.Recent, r.Rescan = 2*time.Second, 2*time.Hour, 30*time.Minute, 10*time.Second
	r.ShowText, r.Port, r.MaxSubscribers, r.Bind = true, "8909", 32, "127.0.0.1"
	r.AllowedHosts = []string{"localhost:8909", "127.0.0.1:8909"}
	for _, k := range []string{"poll", "window", "recent", "rescan", "show_text", "port", "allowed_hosts", "max_subscribers", "bind"} {
		r.Sources[k] = "default"
	}
	if err := r.fromFile(doc); err != nil {
		return err
	}
	r.fromEnv(env)
	if r.Sources["allowed_hosts"] == "default" {
		// Neutral and port-aware: only this machine, on the port in effect.
		r.AllowedHosts = []string{"localhost:" + r.Port, "127.0.0.1:" + r.Port}
	}
	if err := r.uiFromFile(doc); err != nil {
		return err
	}
	return r.sections(env, doc)
}

func (r *Resolved) fromFile(doc *fileDoc) error {
	if doc.Host != nil {
		r.Host, r.Sources["host"] = *doc.Host, "file"
	}
	if doc.Home != nil {
		r.Home, r.Sources["home"] = *doc.Home, "file"
	}
	for _, d := range []struct {
		key string
		in  *string
		to  *time.Duration
	}{{"poll", doc.Poll, &r.Poll}, {"window", doc.Window, &r.Window}, {"recent", doc.Recent, &r.Recent}, {"rescan", doc.Rescan, &r.Rescan}} {
		if d.in == nil {
			continue
		}
		v, err := time.ParseDuration(*d.in)
		if err != nil || v <= 0 {
			return fmt.Errorf("config %s: %s must be a positive duration like \"2s\", got %q", r.Path, d.key, *d.in)
		}
		*d.to, r.Sources[d.key] = v, "file"
	}
	if doc.ShowText != nil {
		r.ShowText, r.Sources["show_text"] = *doc.ShowText, "file"
	}
	return r.serverFromFile(doc.Server)
}

// fromEnv applies the environment over the file. An unparseable value is
// ignored, as it always has been, so a typo cannot stop a deploy.
func (r *Resolved) fromEnv(env Env) {
	if v := env.Get("FLEET_HOST"); v != "" {
		r.Host, r.Sources["host"] = v, "env"
	}
	if v := env.Get("FLEET_HOME"); v != "" {
		r.Home, r.Sources["home"] = v, "env"
	}
	for _, d := range []struct {
		key, name string
		to        *time.Duration
	}{{"poll", "FLEET_POLL", &r.Poll}, {"window", "FLEET_WINDOW", &r.Window}, {"recent", "FLEET_RECENT", &r.Recent}, {"rescan", "FLEET_RESCAN", &r.Rescan}} {
		if v, err := time.ParseDuration(env.Get(d.name)); err == nil && v > 0 {
			*d.to, r.Sources[d.key] = v, "env"
		}
	}
	switch strings.ToLower(env.Get("FLEET_SHOW_TEXT")) {
	case "0", "false", "no", "off":
		r.ShowText, r.Sources["show_text"] = false, "env"
	case "1", "true", "yes", "on":
		r.ShowText, r.Sources["show_text"] = true, "env"
	}
	if v := env.Get("FLEET_PORT"); v != "" {
		r.Port, r.Sources["port"] = v, "env"
	}
	if hosts := lowerAll(strings.Split(env.Get("FLEET_ALLOWED_HOSTS"), ",")); len(hosts) > 0 {
		r.AllowedHosts, r.Sources["allowed_hosts"] = hosts, "env"
	}
	if v := env.Get("FLEET_BIND"); v != "" {
		r.Bind, r.Sources["bind"] = strings.TrimSpace(v), "env"
	}
	if n, err := strconv.Atoi(env.Get("FLEET_MAX_SUBSCRIBERS")); err == nil && n > 0 {
		r.MaxSubscribers, r.Sources["max_subscribers"] = n, "env"
	}
}
