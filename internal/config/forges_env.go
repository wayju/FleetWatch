package config

// Forge defaults and the Gitea environment overrides (see ADR-005). The variables
// that configured the single Gitea before the forges list existed still work:
// they edit the first gitea entry, or build one when the file has none.

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/wayju/fleetwatch/internal/forge"
)

// fillHostAndWeb checks the API URL and fills the host (from the API URL) and
// the web URL (the API URL) when the file left them out.
func fillHostAndWeb(s *forge.Spec) error {
	apiHost, err := checkURL("api_url", s.APIURL)
	if err != nil {
		return err
	}
	if s.Host == "" {
		s.Host = apiHost
	}
	if s.WebURL == "" {
		s.WebURL = s.APIURL
	}
	if _, err := checkURL("web_url", s.WebURL); err != nil {
		return err
	}
	s.APIURL, s.WebURL = strings.TrimSuffix(s.APIURL, "/"), strings.TrimSuffix(s.WebURL, "/")
	return nil
}

// giteaEnv applies GITEA_URL, GITEA_WEB_URL, GITEA_TOKEN, FLEET_GITEA_OWNERS
// and FLEET_GITEA_HOSTS. With none set the forges are untouched. Without a
// gitea entry in the file, GITEA_URL is what creates one.
func (r *Resolved) giteaEnv(env Env) error {
	apiURL, webURL := env.Get("GITEA_URL"), env.Get("GITEA_WEB_URL")
	token, owners, hosts := env.Get("GITEA_TOKEN"), env.Get("FLEET_GITEA_OWNERS"), env.Get("FLEET_GITEA_HOSTS")
	if apiURL == "" && webURL == "" && token == "" && owners == "" && hosts == "" {
		return nil
	}
	idx := -1
	for i, f := range r.Forges {
		if f.Kind == "gitea" {
			idx = i
			break
		}
	}
	if idx < 0 {
		if apiURL == "" {
			return nil // nothing to attach the overrides to
		}
		r.Forges = append(r.Forges, forge.Spec{Kind: "gitea", APIURL: apiURL})
		idx = len(r.Forges) - 1
	}
	f := r.Forges[idx]
	if apiURL != "" {
		f.APIURL = apiURL
		if h, err := url.Parse(apiURL); err == nil && f.Host == "" {
			f.Host = strings.ToLower(h.Hostname())
		}
	}
	if webURL != "" {
		f.WebURL = webURL
	}
	if token != "" {
		f.Token = token
	}
	if owners != "" {
		f.Owners = lowerAll(strings.Split(owners, ","))
	}
	if h := lowerAll(strings.Split(hosts, ",")); len(h) > 0 {
		f.Aliases = nil
		for _, x := range h {
			if x != f.Host {
				f.Aliases = append(f.Aliases, x)
			}
		}
	}
	if err := fillHostAndWeb(&f); err != nil {
		return fmt.Errorf("config %s: gitea environment: %v", r.Path, err)
	}
	r.Forges[idx] = f
	r.Sources["forges"] = "env"
	return nil
}
