// fleetwatch: a live board of every agent session on this host. The service
// itself lives in internal/core; this is only the entry point.
package main

import (
	_ "embed"

	"github.com/wayju/fleetwatch/internal/config"
	"github.com/wayju/fleetwatch/internal/core"
)

// version is the release number; /version reports it unless a build overrides
// it with APP_VERSION or a link-time stamp.
//
//go:embed VERSION
var version string

func main() {
	core.EmbeddedVersion = version
	core.Run(config.OSEnv())
}
