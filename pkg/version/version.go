package version

import (
	"runtime/debug"

	"github.com/go-logr/logr"
)

func GoVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "---"
	}
	return bi.GoVersion
}

func MainVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "---"
	}
	return bi.Main.Version
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
	log.Info("version", "version", MainVersion(), "hash", Hash(), "time", Time())
}
