package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Reading environ, the cwd link, fd or exe under docker-default AppArmor
// floods the audit queue (see ADR-007). No runtime adapter may even name them; the
// next adapters (tmux, herdr) inherit this test.
func TestRuntimeAdaptersNeverTouchPtraceMediatedProcFiles(t *testing.T) {
	banned := []string{`"environ"`, `"cwd"`, `"fd"`, `"exe"`, "os.Readlink"}
	err := filepath.Walk(hostDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for n, line := range strings.Split(string(b), "\n") {
			if strings.Contains(line, `json:"`) { // a scanner file's field name, not a /proc path
				continue
			}
			for _, w := range banned {
				if strings.Contains(line, w) {
					t.Errorf("%s:%d contains %s", path, n+1, w)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
