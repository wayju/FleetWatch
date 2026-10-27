package core

import (
	"fmt"
	"github.com/wayju/fleetwatch/internal/config"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tokenEnv(vars map[string]string) config.Env {
	return config.Env{
		Get:         func(k string) string { return vars[k] },
		UserHomeDir: func() (string, error) { return "/nonexistent-home", nil },
	}
}

func TestEnvTokenWinsAndWritesNothing(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveToken(tokenEnv(map[string]string{"FLEET_TOKEN": "from-env", "XDG_STATE_HOME": dir}), func(string, ...any) {})
	if err != nil || got != "from-env" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fleet")); !os.IsNotExist(err) {
		t.Fatalf("env token must not create the state dir: %v", err)
	}
}

func TestGenerateReuseAndPerms(t *testing.T) {
	dir := t.TempDir()
	env := tokenEnv(map[string]string{"XDG_STATE_HOME": dir})
	var logs []string
	logf := func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }

	first, err := resolveToken(env, logf)
	if err != nil || len(first) != 64 {
		t.Fatalf("generate: %q, %v", first, err)
	}
	path := filepath.Join(dir, "fleet", "token")
	if st, err := os.Stat(path); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v, %v", st, err)
	}
	second, err := resolveToken(env, logf)
	if err != nil || second != first {
		t.Fatalf("reuse: %q vs %q, %v", second, first, err)
	}
	for _, l := range logs {
		if strings.Contains(l, first) {
			t.Fatalf("log leaks the token: %q", l)
		}
		if !strings.Contains(l, path) {
			t.Errorf("log should name the path: %q", l)
		}
	}
}

func TestReuseTightensLoosePerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fleet", "token")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("kept\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := resolveToken(tokenEnv(map[string]string{"XDG_STATE_HOME": dir}), func(string, ...any) {})
	if err != nil || got != "kept" {
		t.Fatalf("got %q, %v", got, err)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", st.Mode().Perm())
	}
}

func TestStateFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	env := tokenEnv(nil)
	env.UserHomeDir = func() (string, error) { return home, nil }
	if _, err := resolveToken(env, func(string, ...any) {}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "state", "fleet", "token")); err != nil {
		t.Fatal(err)
	}
}

func TestSymlinkedTokenFileIsNotFollowed(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "planted")
	if err := os.WriteFile(target, []byte("planted-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fleet", "token")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
	got, err := resolveToken(tokenEnv(map[string]string{"XDG_STATE_HOME": dir}), func(string, ...any) {})
	if err == nil || got == "planted-token" {
		t.Fatalf("followed the symlink: got %q, %v", got, err)
	}
}
