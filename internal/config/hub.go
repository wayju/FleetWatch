package config

// The hub section (see ADR-008): this instance merges the boards of other fleet
// instances into one, optionally with its own host. A token is never in the
// file; token_file names where to read it, and it is read once at startup and
// never logged.

import (
	"fmt"
	"strings"
)

// Upstream is one fleet instance a hub subscribes to.
type Upstream struct {
	Name  string
	URL   string // base URL, no trailing slash; /v1/events is appended
	Token string // read from token_file; never logged
}

// Bounds for one event read from an upstream. The default covers a very large
// board with room to spare; a bigger one is a broken or hostile upstream.
const (
	DefaultMaxEventBytes = 8 << 20
	minMaxEventBytes     = 64 << 10
	maxMaxEventBytes     = 256 << 20
)

// HubSpec is the resolved hub section.
type HubSpec struct {
	Upstreams []Upstream
	Local     bool // this instance also collects its own host
	// MaxEventBytes bounds one event read from an upstream; never zero.
	MaxEventBytes int
}

type hubDoc struct {
	Upstreams []struct {
		Name          string `json:"name"`
		URL           string `json:"url"`
		TokenFile     string `json:"token_file"`
		AllowInsecure bool   `json:"allow_insecure"`
	} `json:"upstreams"`
	Local         bool `json:"local"`
	MaxEventBytes *int `json:"max_event_bytes"`
}

func (r *Resolved) hubFromFile(env Env, doc *fileDoc) error {
	if doc == nil || !present(doc.Hub) {
		return nil
	}
	var d hubDoc
	if err := strict(doc.Hub, &d); err != nil {
		return fmt.Errorf("config %s: hub: %v", r.Path, err)
	}
	if len(d.Upstreams) == 0 && !d.Local {
		return fmt.Errorf("config %s: hub needs at least one upstream, or local true", r.Path)
	}
	h := &HubSpec{Local: d.Local, MaxEventBytes: DefaultMaxEventBytes}
	if m := d.MaxEventBytes; m != nil {
		if *m < minMaxEventBytes || *m > maxMaxEventBytes {
			return fmt.Errorf("config %s: hub.max_event_bytes must be between %d and %d", r.Path, minMaxEventBytes, maxMaxEventBytes)
		}
		h.MaxEventBytes = *m
	}
	seen := map[string]bool{}
	for i, u := range d.Upstreams {
		fail := func(f string, a ...any) error {
			return fmt.Errorf("config %s: hub.upstreams[%d]: %s", r.Path, i, fmt.Sprintf(f, a...))
		}
		name := strings.TrimSpace(u.Name)
		switch {
		case name == "":
			return fail("name is required")
		case seen[name]:
			return fail("name %q is used twice", name)
		case u.TokenFile == "":
			return fail("token_file is required; tokens are never inline")
		}
		seen[name] = true
		if err := r.checkTransport("url", u.URL, u.AllowInsecure); err != nil {
			return fail("%v", err)
		}
		b, err := env.ReadFile(r.expand(u.TokenFile))
		if err != nil {
			return fail("token_file: %v", err)
		}
		tok := strings.TrimSpace(string(b))
		if tok == "" {
			return fail("token_file is empty")
		}
		h.Upstreams = append(h.Upstreams, Upstream{Name: name, URL: strings.TrimRight(u.URL, "/"), Token: tok})
	}
	r.Hub, r.Sources["hub"] = h, "file"
	return nil
}

// hubDescribe is the startup-log line: upstream names and URLs, never tokens.
func (r *Resolved) hubDescribe() string {
	if r.Hub == nil {
		return "off"
	}
	parts := []string{fmt.Sprintf("local=%v", r.Hub.Local)}
	for _, u := range r.Hub.Upstreams {
		parts = append(parts, u.Name+"="+u.URL)
	}
	return strings.Join(parts, " ")
}
