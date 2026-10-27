package config

import (
	"strings"
	"testing"
)

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "LOCALHOST": true, "app.localhost": true, "127.0.0.1": true, "127.9.8.7": true, "::1": true,
		"example.com": false, "localhost.example.com": false, "10.0.0.5": false, "192.0.2.1": false, "0.0.0.0": false,
	} {
		if got := isLoopbackHost(host); got != want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func upstreamDoc(url, extra string) string {
	return `{"upstreams":[{"name":"b","url":"` + url + `","token_file":"/etc/fleet/a.token"` + extra + `}]}`
}

func TestHubPlainHTTPPolicy(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1:8909", "http://localhost:8909", "http://[::1]:8909", "https://b.example"} {
		vars, files := hubDocWith(upstreamDoc(u, ""))
		if r := mustResolve(t, vars, files); len(r.Warnings) != 0 {
			t.Errorf("%s: unexpected warnings %v", u, r.Warnings)
		}
	}
	vars, files := hubDocWith(upstreamDoc("http://b.example:8909", ""))
	failsWith(t, vars, files, "allow_insecure")

	vars, files = hubDocWith(upstreamDoc("http://b.example:8909", `,"allow_insecure":true`))
	r := mustResolve(t, vars, files)
	if len(r.Warnings) != 1 || !strings.Contains(r.Warnings[0], "b.example") || strings.Contains(r.Warnings[0], "tok-a") {
		t.Fatalf("warnings %v", r.Warnings)
	}
}

func recapDoc(url, extra string) string {
	return `{"version":1,"recap":{"url":"` + url + `","model":"m"` + extra + `},"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`
}

func TestRecapPlainHTTPPolicy(t *testing.T) {
	vars, files := withFile(recapDoc("http://127.0.0.1:4000", ""))
	mustResolve(t, vars, files)

	vars, files = withFile(recapDoc("http://gw.example:4000", ""))
	failsWith(t, vars, files, "recap.url")

	vars, files = withFile(recapDoc("http://gw.example:4000", `,"allow_insecure":true`))
	if r := mustResolve(t, vars, files); len(r.Warnings) != 1 {
		t.Fatalf("warnings %v", r.Warnings)
	}

	// The environment URL gets the same check.
	vars, files = withFile(recapDoc("https://gw.example", ""))
	vars["FLEET_RECAP_URL"] = "http://gw.example:4000"
	failsWith(t, vars, files, "recap.url")
}

func TestHubMaxEventBytes(t *testing.T) {
	vars, files := hubDocWith(upstreamDoc("https://b.example", ""))
	if r := mustResolve(t, vars, files); r.Hub.MaxEventBytes != DefaultMaxEventBytes || DefaultMaxEventBytes != 8<<20 {
		t.Fatalf("default %d", r.Hub.MaxEventBytes)
	}
	body := func(n string) string {
		return `{"max_event_bytes":` + n + `,"upstreams":[{"name":"b","url":"https://b.example","token_file":"/etc/fleet/a.token"}]}`
	}
	vars, files = hubDocWith(body("1048576"))
	if r := mustResolve(t, vars, files); r.Hub.MaxEventBytes != 1<<20 {
		t.Fatalf("set %d", r.Hub.MaxEventBytes)
	}
	for _, n := range []string{"1000", "0", "-5", "999999999999"} {
		vars, files = hubDocWith(body(n))
		failsWith(t, vars, files, "max_event_bytes")
	}
}

func TestServerMemoryLimit(t *testing.T) {
	if r := mustResolve(t, nil, nil); r.MemoryLimitMiB != 0 {
		t.Fatalf("default %d", r.MemoryLimitMiB)
	}
	doc := func(n string) string {
		return `{"version":1,"server":{"memory_limit_mib":` + n + `},"runtimes":[{"type":"proc"}],"agents":[{"type":"claudecode"}]}`
	}
	vars, files := withFile(doc("512"))
	if r := mustResolve(t, vars, files); r.MemoryLimitMiB != 512 || r.Sources["memory_limit_mib"] != "file" {
		t.Fatalf("set %d %v", r.MemoryLimitMiB, r.Sources)
	}
	vars, files = withFile(doc("-1"))
	failsWith(t, vars, files, "memory_limit_mib")
	vars, files = withFile(doc(`"big"`))
	failsWith(t, vars, files, "memory_limit_mib")
}
