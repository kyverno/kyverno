package version

import (
	"github.com/golang/glog"
)

// These fields are set during an official build
// Global vars set from command-line arguments
var (
	BuildVersion = "--"
	BuildHash    = "--"
	BuildTime    = "--"
)

//PrintVersionInfo displays the kyverno version - git version
func PrintVersionInfo() {
	glog.Infof("Kyverno version: %s\n", BuildVersion)
	glog.Infof("Kyverno BuildHash: %s\n", BuildHash)
	glog.Infof("Kyverno BuildTime: %s\n", BuildTime)
}
