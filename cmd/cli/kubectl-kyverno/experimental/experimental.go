package experimental

import (
	"os"
	"strconv"
)

const (
	ExperimentalEnv    = "KYVERNO_EXPERIMENTAL"
	KubectlValidateEnv = "KYVERNO_KUBECTL_VALIDATE" //nolint:gosec
)

func getBool(env string) bool {
	if b, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		return b
	}
	return false
}

func IsEnabled() bool {
	return getBool(ExperimentalEnv)
}

func UseKubectlValidate() bool {
	return getBool(KubectlValidateEnv)
}
