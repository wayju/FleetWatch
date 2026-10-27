package config

import (
	"os"
	"path/filepath"
	"testing"
)

// optionalExamples names shipped examples that need an optional adapter; a
// build that carries the adapter adds them in its own test file.
var optionalExamples = map[string]bool{}

// Every shipped example config must load under the strict loader, or the
// README's starting points rot. References to other files (the tmux
// rules) resolve to the shipped example of that file. The example for an
// optional adapter is loaded by cmd/fleetwatch's tests, which carry it.
func TestShippedExampleConfigsLoad(t *testing.T) {
	files, err := filepath.Glob("../../examples/config/*.json")
	if err != nil || len(files) < 6 {
		t.Fatalf("examples: %v %v", files, err)
	}
	for _, f := range files {
		if optionalExamples[filepath.Base(f)] {
			continue
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			r, err := Resolve(Env{
				Get: func(k string) string {
					if k == "FLEET_CONFIG" {
						return f
					}
					return ""
				},
				UserHomeDir: func() (string, error) { return "/home/x", nil },
				Hostname:    func() (string, error) { return "h", nil },
				ReadFile: func(p string) ([]byte, error) {
					if p == f {
						return os.ReadFile(p)
					}
					if filepath.Base(p) == "tmux-rules.json" {
						return os.ReadFile("../../examples/tmux-rules.claude-code.json")
					}
					if filepath.Ext(p) == ".token" {
						return []byte("example-token\n"), nil
					}
					return nil, os.ErrNotExist
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if filepath.Base(f) == "full.json" && (r.Models.Len() == 0 || r.RecapModel == "" || r.PricesFile == "") {
				t.Fatalf("example sections not read: %+v", r)
			}
		})
	}
}
