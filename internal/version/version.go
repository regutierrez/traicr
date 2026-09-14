package version

import "fmt"

var (
	buildVersion = "development"
	buildCommit  = "unknown"
	buildDate    = "unknown"
)

type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

func CurrentBuildInfo() BuildInfo {
	return BuildInfo{
		Version:   buildVersion,
		Commit:    buildCommit,
		BuildDate: buildDate,
	}
}

func (info BuildInfo) String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", info.Version, info.Commit, info.BuildDate)
}
