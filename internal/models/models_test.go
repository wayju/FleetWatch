package models

import "testing"

func table(t *testing.T) *Table {
	t.Helper()
	tbl, err := New([]Entry{
		{Match: "m-*\\[1m\\]", ContextWindow: 1000, Class: "wide"},
		{Match: "m-*", ContextWindow: 100, ExtendedWindow: 1000, Class: "std"},
		{Match: "other", ContextWindow: 50},
	})
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

func TestFirstMatchWins(t *testing.T) {
	tbl := table(t)
	if got := tbl.Class("m-1[1m]"); got != "wide" {
		t.Errorf("class %q, want wide", got)
	}
	if got := tbl.Class("m-1"); got != "std" {
		t.Errorf("class %q, want std", got)
	}
	if got := tbl.Class("other"); got != "" {
		t.Errorf("class %q, want empty", got)
	}
}

func TestWindow(t *testing.T) {
	tbl := table(t)
	for _, c := range []struct {
		model  string
		tokens int64
		want   int64
	}{{"m-1", 50, 100}, {"m-1", 100, 100}, {"m-1", 101, 1000}, {"m-1[1m]", 5, 1000}, {"other", 500, 50}, {"unknown", 5, 0}, {"", 5, 0}} {
		if got := tbl.Window(c.model, c.tokens); got != c.want {
			t.Errorf("%s/%d: window %d, want %d", c.model, c.tokens, got, c.want)
		}
	}
}

func TestNilTableKnowsNothing(t *testing.T) {
	var tbl *Table
	if tbl.Window("m-1", 5) != 0 || tbl.Class("m-1") != "" || tbl.Len() != 0 {
		t.Fatal("a nil table should match nothing")
	}
}

func TestNewRejectsBadEntries(t *testing.T) {
	for name, e := range map[string]Entry{
		"no match": {ContextWindow: 1}, "bad glob": {Match: "[", ContextWindow: 1},
		"no window": {Match: "a"}, "extended not larger": {Match: "a", ContextWindow: 5, ExtendedWindow: 5},
	} {
		if _, err := New([]Entry{e}); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}
