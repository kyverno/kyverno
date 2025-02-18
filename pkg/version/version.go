package version

import (
	"runtime/debug"

	"github.com/go-logr/logr"
)

// BuildVersion is provided by govvv at compile-time
var BuildVersion string

func Version() string {
	if BuildVersion == "" {
		bi, ok := debug.ReadBuildInfo()
		if !ok {
			return "---"
		}
		BuildVersion = bi.Main.Version
	}
	return BuildVersion
}

func Time() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.time" {
				return setting.Value
			}
		}
	}
	return "---"
}

func Hash() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range bi.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "---"
}

// PrintVersionInfo displays the kyverno version - git version
func PrintVersionInfo(log logr.Logger) {
	log.V(2).Info("version", "version", Version(), "hash", Hash(), "time", Time())
}
