package core

import (
	"testing"
)

func TestBranchPatternIsConfigurable(t *testing.T) {
	neutral := newIssueScan("")
	house := newIssueScan(`^(?:feature|fix|chore)/(\d+)-`)
	cases := []struct {
		branch string
		n, h   int
	}{
		{"feature/3431-forge", 3431, 3431},
		{"123_thing", 123, 0},
		{"user/45-x", 45, 0},
		{"claude/practical-euler-x6rbs7", 0, 0},
		{"fix/99999999-too-big", 0, 0},
	}
	for _, c := range cases {
		if got := neutral.branchIssue(c.branch); got != c.n {
			t.Errorf("neutral %q = %d, want %d", c.branch, got, c.n)
		}
		if got := house.branchIssue(c.branch); got != c.h {
			t.Errorf("house %q = %d, want %d", c.branch, got, c.h)
		}
	}
}
