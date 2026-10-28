// Package ui embeds the browser page: the built output of the web/ library
// (index.html, fleet.js, fleet.css and theme.css). The files are committed, so
// the Go build needs no Node; CI rebuilds them and fails on any difference.
package ui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var dist embed.FS

// Files is the embedded page, rooted so that "index.html" is a top-level name.
func Files() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Unreachable: "dist" is embedded above, and a missing directory
		// would have stopped the build.
		panic(err)
	}
	return sub
}
