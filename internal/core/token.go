// token.go: where the bearer token comes from. FLEET_TOKEN wins; with
// none set, a random token is generated on first run and kept in the state
// directory (0600) so later runs reuse it. The log names the path, never the
// value, so an unconfigured instance is closed rather than open.
package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/fsafe"
	"os"
	"path/filepath"
	"strings"
)

// tokenPath is $XDG_STATE_HOME/fleet/token, else ~/.local/state/fleet/token.
func tokenPath(env config.Env) (string, error) {
	base := env.Get("XDG_STATE_HOME")
	if base == "" {
		home, err := env.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "fleet", "token"), nil
}

// resolveToken returns the token to guard the data routes with. logf receives
// where the token lives, never its value. An error means no token could be
// obtained; the caller then serves 503 on the data routes.
func resolveToken(env config.Env, logf func(string, ...any)) (string, error) {
	if t := env.Get("FLEET_TOKEN"); t != "" {
		return t, nil
	}
	path, err := tokenPath(env)
	if err != nil {
		return "", err
	}
	// ReadSmall refuses a symlink, FIFO or oversized file planted at the path.
	if b, err := fsafe.ReadSmall(path, 4096); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			_ = os.Chmod(path, 0o600) // a copied-in file may be looser than we write it
			logf("FLEET_TOKEN not set: using the generated token in %s", path)
			return t, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	t, err := generateToken(path)
	if err != nil {
		return "", err
	}
	logf("FLEET_TOKEN not set: generated a token and stored it in %s (mode 0600)", path)
	return t, nil
}

func generateToken(path string) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	t := hex.EncodeToString(buf)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	// Write to a private temp file in the same directory, then rename, so a
	// reader never sees a half-written token and the file is never wider than
	// 0600 (CreateTemp opens it 0600 and picks an unpredictable name).
	f, err := os.CreateTemp(filepath.Dir(path), ".token-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_, werr := f.WriteString(t + "\n")
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return "", werr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return t, nil
}
