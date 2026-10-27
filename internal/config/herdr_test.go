package config

import "testing"

const herdrAgents = `"agents":[{"type":"claudecode"}]`

func TestHerdrRuntimeDefaults(t *testing.T) {
	vars, files := withFile(`{"version":1,"runtimes":[{"type":"proc"},{"type":"herdr"}],` + herdrAgents + `}`)
	r := mustResolve(t, vars, files)
	h := spec(r.Runtimes[1])
	if h.Type != "herdr" || h.Socket != "/home/proc/.config/herdr/herdr.sock" || h.Rank != 25 {
		t.Fatalf("%+v", h)
	}
	if r.Sources["runtimes[1].socket"] != "default" || r.Sources["runtimes[1].rank"] != "default" {
		t.Fatalf("%v", r.Sources)
	}
	rts, _ := r.Build()
	if len(rts) != 2 || rts[1].Name() != "herdr" {
		t.Fatalf("%v", rts)
	}
}

func TestHerdrFileThenEnvAndTildeAgainstFleetHome(t *testing.T) {
	vars, files := withFile(`{"version":1,"home":"/data/home","runtimes":[{"type":"herdr","socket":"~/h.sock","rank":22}],` + herdrAgents + `}`)
	h := spec(mustResolve(t, vars, files).Runtimes[0])
	if h.Socket != "/data/home/h.sock" || h.Rank != 22 {
		t.Fatalf("%+v", h)
	}
	vars["HERDR_SOCKET_PATH"] = "/run/herdr.sock"
	r := mustResolve(t, vars, files)
	if spec(r.Runtimes[0]).Socket != "/run/herdr.sock" || r.Sources["runtimes[0].socket"] != "env" || r.Sources["runtimes[0].rank"] != "file" {
		t.Fatalf("%+v %v", spec(r.Runtimes[0]), r.Sources)
	}
}

func TestHerdrRejectsUnknownKeysAndOutOfBandRanks(t *testing.T) {
	for body, want := range map[string]string{
		`{"type":"herdr","sockett":"/x"}`: "unknown field",
		`{"type":"herdr","token":"abc"}`:  "unknown field",
		`{"type":"herdr","rank":10}`:      "rank must be above 10 and below 30",
		`{"type":"herdr","rank":30}`:      "rank must be above 10 and below 30",
		`{"type":"herdr","rank":"high"}`:  "rank",
	} {
		vars, files := withFile(`{"version":1,"runtimes":[` + body + `],` + herdrAgents + `}`)
		failsWith(t, vars, files, want)
	}
}
