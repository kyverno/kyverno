package experimental

import (
	"os"
	"strconv"
)

const experimentalEnv = "KYVERNO_EXPERIMENTAL"

func IsEnabled() bool {
	if b, err := strconv.ParseBool(os.Getenv(experimentalEnv)); err == nil {
		return b
	}
	return false
}
