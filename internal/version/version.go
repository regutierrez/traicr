// Package version exposes build metadata shared by Traicr applications.
package version

import "fmt"

var (
	buildVersion = "development"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

// BuildInfo identifies one deterministic Traicr build.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// CurrentBuildInfo returns metadata supplied by release linker flags or development defaults.
func CurrentBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   buildVersion,
		Commit:    buildCommit,
		BuildDate: buildDate,
	}
}

// String formats complete build metadata for command-line output.
func (info BuildInfo) String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", info.Version, info.Commit, info.BuildDate)
}
