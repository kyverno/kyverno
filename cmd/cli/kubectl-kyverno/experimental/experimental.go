package experimental

import (
	"os"
	"strconv"
)

const (
	ExperimentalEnv    = "KYVERNO_EXPERIMENTAL"
	KubectlValidateEnv = "KYVERNO_KUBECTL_VALIDATE"
)

func getBool(env string, fallback bool) bool {
	if b, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		return b
	}
	return fallback
}

func IsEnabled() bool {
	return getBool(ExperimentalEnv, false)
}

func UseKubectlValidate() bool {
	return getBool(KubectlValidateEnv, true)
}
