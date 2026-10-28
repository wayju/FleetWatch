// ui.go: the ui section. It switches the embedded browser page and its session
// route on or off, and names an operator's theme stylesheet to serve in place
// of the built-in one. The server validates and reads the theme file once at
// start, not here.
package config

import (
	"fmt"
	"strconv"
)

type uiSection struct {
	Enabled   *bool   `json:"enabled"`
	ThemeFile *string `json:"theme_file"`
}

// uiFromFile applies the ui section. The page is on by default: a standalone
// install with no UI would have nothing to look at.
func (r *Resolved) uiFromFile(doc *fileDoc) error {
	r.UIEnabled = true
	r.Sources["ui.enabled"], r.Sources["ui.theme_file"] = "default", "default"
	if doc == nil || !present(doc.UI) {
		return nil
	}
	var s uiSection
	if err := strict(doc.UI, &s); err != nil {
		return fmt.Errorf("config %s: ui: %v", r.Path, err)
	}
	if s.Enabled != nil {
		r.UIEnabled, r.Sources["ui.enabled"] = *s.Enabled, "file"
	}
	if s.ThemeFile != nil {
		r.UIThemeFile, r.Sources["ui.theme_file"] = r.expand(*s.ThemeFile), "file"
	}
	return nil
}

func (r *Resolved) uiValue(key string) (string, bool) {
	switch key {
	case "ui.enabled":
		return strconv.FormatBool(r.UIEnabled), true
	case "ui.theme_file":
		return r.UIThemeFile, true
	}
	return "", false
}
