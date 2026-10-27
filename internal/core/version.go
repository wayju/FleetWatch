// version.go: what /version reports. The version is the release number, the
// commit is the build.
package core

import "strings"

// productName is the product's public name; the binary, env vars and config
// keys keep the fleet naming.
const productName = "FleetWatch"

// Stamped at link time by release builds (-ldflags "-X ...") and set from the
// embedded VERSION file by main; all three are empty in a plain `go build`.
var (
	BuildVersion    string
	BuildCommit     string
	EmbeddedVersion string
)

// buildInfo resolves the /version body. APP_VERSION (set by an image build)
// wins unless it is empty or "dev"; then the link-time stamp, then the
// embedded VERSION file, then "dev". The commit is APP_COMMIT, then the stamp,
// then "unknown".
func buildInfo(getenv func(string) string) map[string]string {
	version := pick(getenv("APP_VERSION"), BuildVersion, EmbeddedVersion)
	if version == "" {
		version = "dev"
	}
	commit := pick(getenv("APP_COMMIT"), BuildCommit)
	if commit == "" {
		commit = "unknown"
	}
	return map[string]string{"name": productName, "commit": commit, "version": version}
}

// pick returns the first non-blank candidate that is not "dev".
func pick(candidates ...string) string {
	for _, c := range candidates {
		if c = strings.TrimSpace(c); c != "" && c != "dev" {
			return c
		}
	}
	return ""
}
