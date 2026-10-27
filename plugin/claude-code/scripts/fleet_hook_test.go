package scripts

// Tests the fleet-hook.sh helpers by sourcing the script with FLEET_HOOK_LIB
// set, which returns before it needs jq. The end-to-end test needs jq and
// skips without it.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func shOut(t *testing.T, script string, env ...string) string {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestFleetPidValidation(t *testing.T) {
	for in, want := range map[string]string{"123": "123", "": "0", "abc": "0", "12x": "0", "-4": "0", "1 2": "0", "1;2": "0"} {
		got := shOut(t, `. ./fleet-hook.sh; fleet_pid "$P"`, "FLEET_HOOK_LIB=1", "P="+in)
		if got != want {
			t.Errorf("fleet_pid(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFleetNowFormat(t *testing.T) {
	got := shOut(t, `. ./fleet-hook.sh; fleet_now`, "FLEET_HOOK_LIB=1")
	if !regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d{1,9})?Z$`).MatchString(got) {
		t.Fatalf("timestamp %q", got)
	}
	// Where date supports %N the stamp must carry the fraction; elsewhere the
	// whole-second fallback is expected.
	if n := shOut(t, `date -u +%N`); regexp.MustCompile(`^\d+$`).MatchString(n) && !strings.Contains(got, ".") {
		t.Fatalf("date supports %%N but the stamp %q has no fraction", got)
	}
}

func TestHookWritesNumericPidAndFractionalTime(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq not installed")
	}
	state := t.TempDir()
	payload := `{"session_id":"s1","hook_event_name":"Stop"}`
	cmd := exec.Command("sh", "./fleet-hook.sh")
	cmd.Stdin = strings.NewReader(payload)
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+state, "FLEET_PID=not-a-number")
	if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
		t.Fatalf("hook: %v %q", err, out)
	}
	b, err := os.ReadFile(filepath.Join(state, "fleet", "sessions", "s1.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Pid        *int   `json:"pid"`
		ObservedAt string `json:"observed_at"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Pid == nil || *doc.Pid != 0 {
		t.Fatalf("pid not coerced to 0: %s (%v)", b, err)
	}
	if n := shOut(t, `date -u +%N`); regexp.MustCompile(`^\d+$`).MatchString(n) && !strings.Contains(doc.ObservedAt, ".") {
		t.Fatalf("observed_at %q has no fraction", doc.ObservedAt)
	}
}
