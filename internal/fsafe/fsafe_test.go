package fsafe

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestRefusesFIFOAndSymlink(t *testing.T) {
	dir := t.TempDir()
	fifo := filepath.Join(dir, "f.jsonl")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "real.jsonl")
	os.WriteFile(real, []byte("hi"), 0o644)
	link := filepath.Join(dir, "link.jsonl")
	os.Symlink(real, link)
	for name, p := range map[string]string{"fifo": fifo, "symlink": link} {
		if _, err := ReadSmall(p, 100); err == nil {
			t.Errorf("ReadSmall must refuse a %s", name)
		}
		if f, _, err := OpenRegular(p); err == nil {
			f.Close()
			t.Errorf("OpenRegular must refuse a %s", name)
		}
	}
	if b, err := ReadSmall(real, 100); err != nil || string(b) != "hi" {
		t.Errorf("regular file: %q %v", b, err)
	}
	if _, err := ReadSmall(real, 1); err == nil {
		t.Error("over the limit must be refused")
	}
}
