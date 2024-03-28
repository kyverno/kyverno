package log

import (
	"os"
	"strconv"
	"strings"

	"github.com/kyverno/kyverno/pkg/logging"
)

const loggerName = "kubectl-kyverno"

var Log = logging.WithName(loggerName)

func Configure() error {
	return configure(os.Args[1:]...)
}

func configure(args ...string) error {
	logging.InitFlags(nil)

	if isVerbose(args...) {
		if level, err := getLogLevel(args...); err == nil {
			return logging.Setup(logging.TextFormat, logging.DefaultTime, level)
		} else {
			// Use the default log level i.e 0 to handle the error while extracting level from cli
			return logging.Setup(logging.TextFormat, logging.DefaultTime, 0)
		}
	}

	return nil
}

func isVerbose(args ...string) bool {
	for _, arg := range args {
		if arg == "-v" || arg == "--v" {
			return true
		} else if strings.HasPrefix(arg, "-v=") || strings.HasPrefix(arg, "--v=") {
			return true
		}
	}
	return false
}

func getLogLevel(args ...string) (int, error) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-v=") {
			levelStr := strings.TrimPrefix(arg, "-v=")
			level, err := strconv.Atoi(levelStr)
			if err != nil {
				// Return an error if conversion fails
				return 0, err
			}
			return level, nil
		} else if strings.HasPrefix(arg, "--v=") {
			levelStr := strings.TrimPrefix(arg, "--v=")
			level, err := strconv.Atoi(levelStr)
			if err != nil {
				// Return an error if conversion fails
				return 0, err
			}
			return level, nil
		}
	}
	return 0, nil
}
