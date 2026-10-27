package policy

// CI guard (see ADR-005): no model name is compiled into the binary. Models, prices
// and context windows are config, so a release does not go stale the day a
// model ships and does not favour one provider. Comments count: a name in a
// comment is a name someone will paste into code.
//
// internal/fleettest is exempt: it is test-support code that fabricates
// transcripts, not part of the shipped binary.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var modelFamily = regexp.MustCompile(`(?i)opus|sonnet|haiku|fable|gpt-|gemini`)

func TestNoModelFamilyLiteralsInShippedGo(t *testing.T) {
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		slash := filepath.ToSlash(rel)
		if d.IsDir() {
			if d.Name() == "testdata" || slash == "internal/fleettest" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if m := modelFamily.FindString(line); m != "" {
				t.Errorf("%s:%d names a model family (%q); models belong in config, not code", slash, i+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModelFamilyPatternCatchesTheFamilies(t *testing.T) {
	for _, s := range []string{"claude-Opus-5", "a sonnet b", "haiku", "fable", "gpt-5", "Gemini-2"} {
		if !modelFamily.MatchString(s) {
			t.Errorf("%q should match", s)
		}
	}
}
