package internal

import (
	"flag"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
)

func setupLogger() logr.Logger {
	logLevel, err := strconv.Atoi(flag.Lookup("v").Value.String())
	checkErr(err, "failed to setup logger")
	checkErr(logging.Setup(loggingFormat, logLevel), "failed to setup logger")
	return logging.WithName("setup")
}
