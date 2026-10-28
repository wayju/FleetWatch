package core

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func get(t *testing.T, g *uiRig, path string) (*http.Response, string) {
	t.Helper()
	res, err := g.srv.Client().Get(g.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, string(b)
}

func TestPageIsServedWithoutAuthUnderStrictCSP(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	res, body := get(t, g, "/")
	if res.StatusCode != 200 || !strings.Contains(body, "<title>FleetWatch</title>") {
		t.Fatalf("GET /: %d %.80q", res.StatusCode, body)
	}
	want := "default-src 'none'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	for k, v := range map[string]string{
		"Content-Security-Policy": want, "X-Content-Type-Options": "nosniff",
		"Referrer-Policy": "no-referrer", "Content-Type": "text/html; charset=utf-8",
	} {
		if got := res.Header.Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestAssetsAreServedWithTheirTypes(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true}, testToken, false)
	for path, ct := range map[string]string{
		"/ui/fleet.js": "text/javascript; charset=utf-8", "/ui/fleet.css": "text/css; charset=utf-8",
		"/ui/theme.css": "text/css; charset=utf-8", "/ui/index.html": "text/html; charset=utf-8",
	} {
		res, body := get(t, g, path)
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != ct || body == "" {
			t.Errorf("%s: %d %q (%d bytes)", path, res.StatusCode, res.Header.Get("Content-Type"), len(body))
		}
		if res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("%s lacks the security headers", path)
		}
	}
	for _, path := range []string{"/ui/missing.js", "/ui/embed.go", "/ui/", "/ui/..%2fVERSION"} {
		if res, _ := get(t, g, path); res.StatusCode != 404 && res.StatusCode != 301 {
			t.Errorf("%s: %d, want 404", path, res.StatusCode)
		}
	}
	if res, _ := get(t, g, "/nowhere"); res.StatusCode != 401 {
		t.Errorf("an unknown path still needs credentials: %d", res.StatusCode)
	}
}

func TestThemeFileReplacesTheBuiltInTheme(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "mine.css")
	if err := os.WriteFile(file, []byte(":root{--fl-night:#123456}"), 0o600); err != nil {
		t.Fatal(err)
	}
	g := newUIRig(t, uiOptions{Enabled: true, ThemeFile: file}, testToken, false)
	if _, body := get(t, g, "/ui/theme.css"); body != ":root{--fl-night:#123456}" {
		t.Errorf("theme.css = %q, want the operator's file", body)
	}
	if _, body := get(t, g, "/ui/fleet.css"); strings.Contains(body, "#123456") {
		t.Error("only theme.css is replaced")
	}
}

func TestMissingThemeFileFallsBackToTheBuiltIn(t *testing.T) {
	g := newUIRig(t, uiOptions{Enabled: true, ThemeFile: filepath.Join(t.TempDir(), "absent.css")}, testToken, false)
	res, body := get(t, g, "/ui/theme.css")
	if res.StatusCode != 200 || !strings.Contains(body, "--fl-night") {
		t.Errorf("fallback: %d %.60q", res.StatusCode, body)
	}
}

func TestThemeFileIsValidatedAtStart(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.css")
	os.WriteFile(real, []byte(":root{--fl-night:#111111}"), 0o600)
	link := filepath.Join(dir, "link.css")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "theme.txt")
	os.WriteFile(txt, []byte(":root{--fl-night:#111111}"), 0o600)
	big := filepath.Join(dir, "big.css")
	os.WriteFile(big, []byte(strings.Repeat("a", themeMax+1)), 0o600)
	for name, p := range map[string]string{"symlink": link, "wrong extension": txt, "oversize": big, "directory": dir} {
		g := newUIRig(t, uiOptions{Enabled: true, ThemeFile: p}, testToken, false)
		if _, body := get(t, g, "/ui/theme.css"); strings.Contains(body, "#111111") || !strings.Contains(body, "--fl-night") {
			t.Errorf("%s: the theme file was served or the fallback is missing", name)
		}
	}
	os.WriteFile(real, []byte(":root{--fl-night:#222222}"), 0o600)
	g := newUIRig(t, uiOptions{Enabled: true, ThemeFile: real}, testToken, false)
	os.WriteFile(real, []byte(":root{--fl-night:#333333}"), 0o600)
	if _, body := get(t, g, "/ui/theme.css"); !strings.Contains(body, "#222222") {
		t.Errorf("the theme is read once at start, got %q", body)
	}
}
