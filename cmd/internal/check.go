package internal

import (
	"errors"
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
