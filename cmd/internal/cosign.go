package internal

import (
	"os"

	"github.com/google/go-containerregistry/pkg/logs"
)

func setupCosignLogging() {
	if !enableCosignLogging {
		return
	}
	logs.Debug.SetOutput(os.Stderr)
}
