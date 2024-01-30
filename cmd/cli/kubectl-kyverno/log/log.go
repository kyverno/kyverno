package log

import (
	"os"
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
		return logging.Setup(logging.TextFormat, 0)
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
