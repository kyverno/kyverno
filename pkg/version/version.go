package version

// These fields are set during an official build
var (
	BuildVersion = "--"
	BuildHash    = "--"
	BuildTime    = "--"
)

// VersionInfo gets json info about the agent version
type VersionInfo struct {
	BuildVersion string
	BuildHash    string
	BuildTime    string
}

// GetVersion gets the current agent version
func GetVersion() *VersionInfo {
	return &VersionInfo{
		BuildVersion: BuildVersion,
		BuildHash:    BuildHash,
		BuildTime:    BuildTime,
	}
}
