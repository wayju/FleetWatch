package config

import "testing"

func TestUIDefaultsToOnWithTheBuiltInTheme(t *testing.T) {
	r := mustResolve(t, nil, nil)
	if !r.UIEnabled || r.UIThemeFile != "" {
		t.Errorf("zero config: enabled=%v theme=%q, want on and no theme file", r.UIEnabled, r.UIThemeFile)
	}
	if r.Sources["ui.enabled"] != "default" || r.Sources["ui.theme_file"] != "default" {
		t.Errorf("sources: %v", r.Sources)
	}
}

func TestUISectionFromFile(t *testing.T) {
	vars, files := withFile(`{"version":1,"ui":{"enabled":false,"theme_file":"~/fleet/theme.css"}}`)
	r := mustResolve(t, vars, files)
	if r.UIEnabled || r.UIThemeFile != "/home/proc/fleet/theme.css" {
		t.Errorf("enabled=%v theme=%q, want off and the expanded path", r.UIEnabled, r.UIThemeFile)
	}
	if r.Sources["ui.enabled"] != "file" || r.Sources["ui.theme_file"] != "file" {
		t.Errorf("sources: %v", r.Sources)
	}
	vars, files = withFile(`{"version":1,"ui":{"theme_file":"/etc/fleet/theme.css"}}`)
	if r := mustResolve(t, vars, files); !r.UIEnabled || r.UIThemeFile != "/etc/fleet/theme.css" {
		t.Errorf("a theme alone leaves the page on: enabled=%v theme=%q", r.UIEnabled, r.UIThemeFile)
	}
}

func TestUISectionIsStrict(t *testing.T) {
	for name, body := range map[string]string{
		"unknown key": `{"version":1,"ui":{"enable":true}}`,
		"wrong type":  `{"version":1,"ui":{"enabled":"yes"}}`,
	} {
		vars, files := withFile(body)
		t.Run(name, func(t *testing.T) { failsWith(t, vars, files, "ui") })
	}
}
