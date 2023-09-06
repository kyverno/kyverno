package log

import "github.com/kyverno/kyverno/pkg/logging"

const loggerName = "kubectl-kyverno"

var Log = logging.WithName(loggerName)

func Configure() error {
	logging.InitFlags(nil)
	return logging.Setup(logging.TextFormat, 0)
}
