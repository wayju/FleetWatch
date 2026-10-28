package ui

import (
	"io/fs"
	"strings"
	"testing"
)

// The page must stay free of inline script and style, or the strict policy
// the server sends would block it.
func TestEmbeddedPageIsSelfContained(t *testing.T) {
	files := Files()
	for _, name := range []string{"index.html", "fleet.js", "fleet.css", "theme.css"} {
		b, err := fs.ReadFile(files, name)
		if err != nil || len(b) == 0 {
			t.Fatalf("%s: %v (%d bytes)", name, err, len(b))
		}
	}
	html, _ := fs.ReadFile(files, "index.html")
	s := string(html)
	for i := strings.Index(s, "<script"); i >= 0; i = nextScript(s, i) {
		end := strings.Index(s[i:], ">")
		if !strings.Contains(s[i:i+end], " src=") {
			t.Errorf("inline script element in index.html")
		}
	}
	if strings.Contains(s, "<style") {
		t.Errorf("inline style element in index.html")
	}
}

func nextScript(s string, from int) int {
	j := strings.Index(s[from+1:], "<script")
	if j < 0 {
		return -1
	}
	return from + 1 + j
}
