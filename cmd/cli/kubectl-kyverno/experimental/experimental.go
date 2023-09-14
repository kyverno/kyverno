package experimental

import (
	"os"
	"strconv"
)

const (
	experimentalEnv    = "KYVERNO_EXPERIMENTAL"
	kubectlValidateEnv = "KYVERNO_KUBECTL_VALIDATE" //nolint:gosec
)

func getBool(env string) bool {
	if b, err := strconv.ParseBool(os.Getenv(env)); err == nil {
		return b
	}
	return false
}

func IsEnabled() bool {
	return getBool(experimentalEnv)
}

func UseKubectlValidate() bool {
	return getBool(kubectlValidateEnv)
}
