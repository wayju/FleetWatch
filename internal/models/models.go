// Package models is the config-driven model table (see ADR-005): an ordered list of
// glob patterns that give a model id its context window and display class.
// No model name is compiled into the binary, so a release does not go stale
// the day a model ships and no provider is favoured.
package models

import (
	"fmt"
	"path"
)

// Entry is one row of the table. The first entry whose Match fits wins, so
// specific patterns go before general ones.
type Entry struct {
	Match          string `json:"match"`                     // path.Match glob over the transcript's model id
	ContextWindow  int64  `json:"context_window"`            // tokens
	ExtendedWindow int64  `json:"extended_window,omitempty"` // window assumed once usage exceeds ContextWindow (an extended-context session reports the same id)
	Class          string `json:"class,omitempty"`           // display class the dashboard colours by
}

// Table is the ordered entries. A nil Table matches nothing.
type Table struct {
	entries []Entry
}

// New validates entries and returns the table.
func New(entries []Entry) (*Table, error) {
	for i, e := range entries {
		if _, err := path.Match(e.Match, ""); err != nil {
			return nil, fmt.Errorf("models[%d]: bad match %q: %v", i, e.Match, err)
		}
		if e.Match == "" {
			return nil, fmt.Errorf("models[%d]: match is required", i)
		}
		if e.ContextWindow <= 0 {
			return nil, fmt.Errorf("models[%d]: context_window must be positive", i)
		}
		if e.ExtendedWindow != 0 && e.ExtendedWindow <= e.ContextWindow {
			return nil, fmt.Errorf("models[%d]: extended_window must exceed context_window", i)
		}
	}
	return &Table{entries: append([]Entry(nil), entries...)}, nil
}

// Lookup returns the first entry matching model.
func (t *Table) Lookup(model string) (Entry, bool) {
	if t == nil || model == "" {
		return Entry{}, false
	}
	for _, e := range t.entries {
		if ok, _ := path.Match(e.Match, model); ok {
			return e, true
		}
	}
	return Entry{}, false
}

// Len is the number of entries.
func (t *Table) Len() int {
	if t == nil {
		return 0
	}
	return len(t.entries)
}

// Class is the display class for model, "" when no entry matches.
func (t *Table) Class(model string) string {
	e, _ := t.Lookup(model)
	return e.Class
}

// Window is the context window for model given the tokens now in context.
// It is 0 when no entry matches (the size is unknown).
func (t *Table) Window(model string, tokens int64) int64 {
	e, ok := t.Lookup(model)
	if !ok {
		return 0
	}
	if e.ExtendedWindow > 0 && tokens > e.ContextWindow {
		return e.ExtendedWindow
	}
	return e.ContextWindow
}
