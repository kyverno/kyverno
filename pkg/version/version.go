package version

import (
	"github.com/go-logr/logr"
)

// These fields are set during an official build
// Global vars set from command-line arguments
var (
	BuildVersion = "--"
	BuildHash    = "--"
	BuildTime    = "--"
)

//PrintVersionInfo displays the kyverno version - git version
func PrintVersionInfo(log logr.Logger) {
	log.Info("Kyverno", "Version", BuildVersion)
	log.Info("Kyverno", "BuildHash", BuildHash)
	log.Info("Kyverno", "BuildTime", BuildTime)
}
