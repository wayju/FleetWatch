// ui.go: the embedded page and the routes that serve it. The page and its
// assets are static and carry no data, so they need no token; the data routes
// they call do. With the UI switched off none of these routes exist.
package core

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/wayju/fleetwatch/internal/ui"
)

// uiCSP lets the page load only its own script and stylesheets and talk only
// to its own origin. Style attributes stay allowed because the renderer sets a
// width or hue per element; there is no inline script or style element.
const uiCSP = "default-src 'none'; script-src 'self'; style-src 'self'; style-src-attr 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// themeMax bounds an operator's theme file; a stylesheet larger than this is
// a mistake.
const themeMax = 512 << 10

var contentTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".css":  "text/css; charset=utf-8",
}

// uiOptions is what the routes need from the ui config section.
type uiOptions struct {
	Enabled   bool
	ThemeFile string // served as theme.css when set; the built-in theme otherwise
}

// registerUI adds the page, its assets and the session route to mux. It does
// nothing when the UI is off, so those paths answer 404.
func registerUI(mux *http.ServeMux, opt uiOptions, token string, sessions *sessionStore) {
	if !opt.Enabled {
		return
	}
	files := ui.Files()
	var theme []byte
	if opt.ThemeFile != "" {
		var err error
		if theme, err = loadTheme(opt.ThemeFile); err != nil {
			log.Printf("ui.theme_file %s: %v; serving the built-in theme", opt.ThemeFile, err)
			theme = nil
		}
	}
	serve := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { serveAsset(w, files, name, theme) }
	}
	mux.HandleFunc("GET /{$}", serve("index.html"))
	mux.HandleFunc("GET /ui/{name}", func(w http.ResponseWriter, r *http.Request) {
		serveAsset(w, files, r.PathValue("name"), theme)
	})
	sh := sessionHandler(token, sessions)
	mux.Handle("POST "+sessionPath, sh)
	mux.Handle("DELETE "+sessionPath, sh)
}

func serveAsset(w http.ResponseWriter, files fs.FS, name string, theme []byte) {
	h := w.Header()
	h.Set("Content-Security-Policy", uiCSP)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Cache-Control", "no-cache")
	ct, ok := contentTypes[path.Ext(name)]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	body := []byte(nil)
	if name == "theme.css" {
		body = theme
	}
	if body == nil {
		var err error
		if body, err = fs.ReadFile(files, name); err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
	}
	h.Set("Content-Type", ct)
	_, _ = w.Write(body)
}

// loadTheme reads the operator's theme once, at start. It must be a regular
// file (not a symlink, FIFO or device) named *.css and no larger than themeMax,
// so the path cannot be pointed at something else after the config is written.
func loadTheme(p string) ([]byte, error) {
	if strings.ToLower(filepath.Ext(p)) != ".css" {
		return nil, errors.New("not a .css file")
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, errors.New("not a regular file (symlinks are refused)")
	}
	if fi.Size() > themeMax {
		return nil, fmt.Errorf("larger than %d bytes", themeMax)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	if len(b) > themeMax {
		return nil, fmt.Errorf("larger than %d bytes", themeMax)
	}
	return b, nil
}
