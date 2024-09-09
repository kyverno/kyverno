package log

import (
	"os"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const loggerName = "kubectl-kyverno"

var defaultLogLevel = 2

var Log = logging.WithName(loggerName)

func Configure() error {
	return configure(os.Args[1:]...)
}

func configure(args ...string) error {
	logging.InitFlags(nil)

	isVerboseBool, level, err := isVerbose(args...)
	if err != nil {
		return err
	}
	if isVerboseBool {
		return logging.Setup(logging.TextFormat, logging.DefaultTime, level)
	} else {
		log.SetLogger(logr.Discard())
	}
	return nil
}

func isVerbose(args ...string) (bool, int, error) {
	for i, arg := range args {
		if arg == "-v" || arg == "--v" {
			level := defaultLogLevel
			if i+1 < len(args) {
				levelStr := args[i+1]
				levelInt, err := strconv.Atoi(levelStr)
				if err != nil {
					// Return an error if conversion fails
					return false, 0, err
				}
				level = levelInt
			}
			return true, level, nil
		} else if strings.HasPrefix(arg, "-v=") || strings.HasPrefix(arg, "--v=") {
			levelStr := strings.TrimPrefix(arg, "-v=")
			levelStr = strings.TrimPrefix(levelStr, "--v=")
			level, err := strconv.Atoi(levelStr)
			if err != nil {
				// Return an error if conversion fails
				return false, 0, err
			}
			return true, level, nil
		}
	}
	return false, 0, nil
}
