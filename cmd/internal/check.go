package internal

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-logr/logr"
)

func check(logger logr.Logger) {
	checkEnvVar(logger, "KYVERNO_NAMESPACE")
	checkEnvVar(logger, "KYVERNO_SERVICEACCOUNT_NAME")
	checkEnvVar(logger, "KYVERNO_DEPLOYMENT")
	checkEnvVar(logger, "KYVERNO_POD_NAME")
	checkEnvVar(logger, "INIT_CONFIG")
	checkEnvVar(logger, "METRICS_CONFIG")
	checkAutoMemLimitRatio(logger)
	checkRegistryResponseHeaderTimeout(logger)
}

func checkAutoMemLimitRatio(logger logr.Logger) {
	if autoMemLimitEnabled && (autoMemLimitRatio <= 0 || autoMemLimitRatio > 1.0) {
		checkError(logger, fmt.Errorf("value %f is invalid", autoMemLimitRatio), "autoMemLimitRatio must be greater than 0 and less than or equal to 1")
	}
}

func checkRegistryResponseHeaderTimeout(logger logr.Logger) {
	if registryResponseHeaderTimeout < 0 {
		checkError(logger, fmt.Errorf("value %s is invalid", registryResponseHeaderTimeout), "registryResponseHeaderTimeout must be non-negative")
	}
}

func checkEnvVar(logger logr.Logger, name string) {
	checkError(logger, validateEnvVar(name), "please define the environment variable", "name", name)
}

func validateEnvVar(name string) error {
	if os.Getenv(name) == "" {
		return errors.New("environment variable must be defined")
	}
	return nil
}
