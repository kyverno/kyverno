package log

import (
	"os"
	"strings"

	"github.com/kyverno/kyverno/pkg/logging"
)

const loggerName = "kubectl-kyverno"

var Log = logging.WithName(loggerName)

func Configure() error {
	logging.InitFlags(nil)
	verbose := false
	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--v" {
			verbose = true
		} else if strings.HasPrefix(arg, "-v=") || strings.HasPrefix(arg, "--v=") {
			verbose = true
		}
	}
	if verbose {
		return logging.Setup(logging.TextFormat, 0)
	}
	return nil
}
