package config

import (
	"strings"
	"testing"
)

func hubDocWith(section string) (map[string]string, map[string]string) {
	vars, files := withFile(`{"version":1,` + baseDoc + `,"hub":` + section + `}`)
	files["/etc/fleet/a.token"] = " tok-a \n"
	files["/etc/fleet/empty.token"] = "\n"
	return vars, files
}

func TestHubDefaultsOff(t *testing.T) {
	r := mustResolve(t, nil, nil)
	if r.Hub != nil || r.hubDescribe() != "off" {
		t.Fatalf("hub %+v", r.Hub)
	}
}

func TestHubFromFile(t *testing.T) {
	vars, files := hubDocWith(`{"local":true,"upstreams":[{"name":"b","url":"https://b.example:8909/","token_file":"/etc/fleet/a.token"}]}`)
	r := mustResolve(t, vars, files)
	u := r.Hub.Upstreams[0]
	if !r.Hub.Local || u.Name != "b" || u.URL != "https://b.example:8909" || u.Token != "tok-a" || r.Sources["hub"] != "file" {
		t.Fatalf("hub %+v sources %v", r.Hub, r.Sources)
	}
	for _, l := range r.Describe() {
		if strings.Contains(l, "tok-a") {
			t.Fatalf("a token reached the startup log: %s", l)
		}
	}
}

func TestHubRejectsBadInput(t *testing.T) {
	up := func(f string) string { return `{"upstreams":[` + f + `]}` }
	good := `"name":"b","url":"https://b:1","token_file":"/etc/fleet/a.token"`
	cases := map[string]struct{ section, want string }{
		"unknown key":      {`{"local":true,"mode":"x"}`, "unknown field"},
		"unknown upstream": {up(`{` + good + `,"token":"x"}`), "unknown field"},
		"empty":            {`{}`, "at least one upstream"},
		"no name":          {up(`{"url":"https://b:1","token_file":"/etc/fleet/a.token"}`), "name is required"},
		"duplicate":        {up(`{` + good + `},{` + good + `}`), "used twice"},
		"no token file":    {up(`{"name":"b","url":"https://b:1"}`), "token_file is required"},
		"bad url":          {up(`{"name":"b","url":"b:1","token_file":"/etc/fleet/a.token"}`), "http(s) URL"},
		"missing file":     {up(`{"name":"b","url":"https://b:1","token_file":"/nope"}`), "token_file"},
		"empty file":       {up(`{"name":"b","url":"https://b:1","token_file":"/etc/fleet/empty.token"}`), "empty"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			vars, files := hubDocWith(c.section)
			failsWith(t, vars, files, c.want)
		})
	}
}

func TestServerBind(t *testing.T) {
	if r := mustResolve(t, nil, nil); r.Bind != "127.0.0.1" {
		t.Fatalf("default bind %q", r.Bind)
	}
	vars, files := withFile(`{"version":1,` + baseDoc + `,"server":{"bind":"0.0.0.0"}}`)
	if r := mustResolve(t, vars, files); r.Bind != "0.0.0.0" {
		t.Fatalf("file bind %q", r.Bind)
	}
	vars["FLEET_BIND"] = "10.0.0.1"
	if r := mustResolve(t, vars, files); r.Bind != "10.0.0.1" {
		t.Fatalf("env bind %q", r.Bind)
	}
}
