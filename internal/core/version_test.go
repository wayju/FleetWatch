package core

import "testing"

func TestBuildInfoPrecedence(t *testing.T) {
	defer func(a, b, c string) { BuildVersion, BuildCommit, EmbeddedVersion = a, b, c }(BuildVersion, BuildCommit, EmbeddedVersion)
	get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	BuildVersion, BuildCommit, EmbeddedVersion = "", "", ""
	if got := buildInfo(get(nil)); got["name"] != "FleetWatch" {
		t.Fatalf("name: %v", got)
	}
	if got := buildInfo(get(nil)); got["version"] != "dev" || got["commit"] != "unknown" {
		t.Fatalf("empty: %v", got)
	}
	EmbeddedVersion = "0.1.0\n"
	if got := buildInfo(get(map[string]string{"APP_VERSION": "dev", "APP_COMMIT": "abc123"})); got["version"] != "0.1.0" || got["commit"] != "abc123" {
		t.Fatalf("dev env falls back to embedded: %v", got)
	}
	BuildVersion, BuildCommit = "0.2.0", "stamped"
	if got := buildInfo(get(nil)); got["version"] != "0.2.0" || got["commit"] != "stamped" {
		t.Fatalf("stamp: %v", got)
	}
	if got := buildInfo(get(map[string]string{"APP_VERSION": "9.9.9", "APP_COMMIT": "env"})); got["version"] != "9.9.9" || got["commit"] != "env" {
		t.Fatalf("env wins: %v", got)
	}
}
