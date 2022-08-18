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

// PrintVersionInfo displays the kyverno version - git version
func PrintVersionInfo(log logr.Logger) {
	log.V(2).Info("Kyverno", "Version", BuildVersion)
	log.V(2).Info("Kyverno", "BuildHash", BuildHash)
	log.V(2).Info("Kyverno", "BuildTime", BuildTime)
}
