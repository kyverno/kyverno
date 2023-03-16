package kube

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/go-logr/logr"
	"k8s.io/client-go/discovery"
)

var regexVersion = regexp.MustCompile(`v(\d+).(\d+).(\d+)\.*`)

// HigherThanKubernetesVersion compare Kubernetes client version to user given version
func HigherThanKubernetesVersion(client discovery.ServerVersionInterface, log logr.Logger, major, minor, patch int) bool {
	logger := log.WithName("CompareKubernetesVersion")
	serverVersion, err := client.ServerVersion()
	if err != nil {
		logger.Error(err, "Failed to get kubernetes server version")
		return false
	}
	b, err := isVersionHigher(serverVersion.String(), major, minor, patch)
	if err != nil {
		logger.Error(err, "serverVersion", serverVersion.String())
		return false
	}
	return b
}

func isVersionHigher(version string, major int, minor int, patch int) (bool, error) {
	groups := regexVersion.FindStringSubmatch(version)
	if len(groups) != 4 {
		return false, fmt.Errorf("invalid version %s. Expected {major}.{minor}.{patch}", version)
	}
	currentMajor, err := strconv.Atoi(groups[1])
	if err != nil {
		return false, fmt.Errorf("failed to extract major version from %s", version)
	}
	currentMinor, err := strconv.Atoi(groups[2])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}
	currentPatch, err := strconv.Atoi(groups[3])
	if err != nil {
		return false, fmt.Errorf("failed to extract minor version from %s", version)
	}
	if currentMajor < major ||
		(currentMajor == major && currentMinor < minor) ||
		(currentMajor == major && currentMinor == minor && currentPatch <= patch) {
		return false, nil
	}
	return true, nil
}
