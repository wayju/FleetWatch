package core

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "rewrite the current goldens from the current output (never the v0 files)")

func goldenPath(name string) string { return filepath.Join("testdata", "golden", name) }

// compareGolden checks got against testdata/golden/<name>.
func compareGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath(name), got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(goldenPath(name))
	if err != nil {
		t.Fatalf("%v (run go test -run Golden -update to create it)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from the golden:\n%s", name, firstDiff(want, got))
	}
}

// canonical decodes a golden, drops the keys v1 added to every session and issue, and
// re-encodes with sorted keys.
func canonical(t *testing.T, b []byte) []byte {
	t.Helper()
	var doc map[string]map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, snap := range doc {
		sessions, _ := snap["sessions"].([]any)
		for _, s := range sessions {
			m := s.(map[string]any)
			delete(m, "host")
			delete(m, "runtime")
			delete(m, "address")
			delete(m, "agent_version")
			// A repo whose origin no forge serves now links to its own host (see ADR-005).
			issues, _ := m["issues"].([]any)
			for _, is := range issues {
				delete(is.(map[string]any), "url")
			}
		}
	}
	out, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
