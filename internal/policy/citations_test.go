package policy

// CI guard: code comments and docs cite design decisions as ADR-00N, never by
// a number in a private tracker a public reader cannot open.

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// citation matches a tracker reference in citing position: "see #123",
// "(#1234)", "issue 123", "PR #12", or a repo-qualified "owner-repo#123".
var citation = regexp.MustCompile(`(?i)(?:\b(?:refs|see|issue|pr)\s+#?\d{3,4}\b|\(#\d{3,4}\b|[a-z]-[a-z]+#\d{2,5}\b)`)

func TestNoPrivateIssueCitations(t *testing.T) {
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(repoRoot, p)
		slash := filepath.ToSlash(rel)
		if d.IsDir() {
			switch d.Name() {
			case "testdata", ".git", ".gitea":
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "CHANGELOG.md" || d.Name() == ".gitleaks.toml" {
			return nil
		}
		goFile := strings.HasSuffix(p, ".go")
		switch {
		case goFile && !strings.HasSuffix(p, "_test.go"):
		case strings.HasSuffix(p, ".sh"), strings.HasSuffix(p, ".md"),
			strings.HasSuffix(p, ".json"), strings.HasSuffix(p, ".yml"):
		default:
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if goFile {
				// Only comments are scrubbed; string literals may hold test-like data.
				k := strings.Index(line, "//")
				if k < 0 {
					continue
				}
				line = line[k:]
			}
			if m := citation.FindString(line); m != "" {
				t.Errorf("%s:%d cites a private issue (%q); state the reason or cite ADR-00N", slash, i+1, m)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCitationPatternCatchesCitations(t *testing.T) {
	for _, s := range []string{"see #3427", "(#1389)", "refs #123", "issue 1389", "PR #1158", "local-dev-stack#1545"} {
		if !citation.MatchString(s) {
			t.Errorf("%q should match", s)
		}
	}
	for _, s := range []string{"acme/app#123 branch", "color #fff", "ADR-007", "PR #$PR_NUMBER", "# 1. heading", "see ADR-005"} {
		if citation.MatchString(s) {
			t.Errorf("%q should not match", s)
		}
	}
}
