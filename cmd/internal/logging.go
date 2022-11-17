package internal

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
)

func SetupLogger() logr.Logger {
	logLevel, err := strconv.Atoi(flag.Lookup("v").Value.String())
	if err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	if err := logging.Setup(loggingFormat, logLevel); err != nil {
		fmt.Println("failed to setup logger", err)
		os.Exit(1)
	}
	return logging.WithName("setup")
}
