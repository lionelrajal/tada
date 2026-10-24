package cmd

import "fmt"

// These are overridden at build time by the release pipeline, which passes
//
//	-ldflags "-X github.com/lionelrajal/tada/cmd.version=v0.1.0 -X github.com/lionelrajal/tada/cmd.commit=... -X github.com/lionelrajal/tada/cmd.date=..."
//
// A plain `go build` keeps the defaults below.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionString describes the build for `tada --version`. Local builds that
// carry no metadata stay a single word instead of printing placeholder values.
func versionString() string {
	if commit == "none" && date == "unknown" {
		return version
	}
	return fmt.Sprintf("%s (commit %s, built %s)", version, commit, date)
}
