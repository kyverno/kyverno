package log

import (
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/klog/v2"
)

const loggerName = "kubectl-kyverno"

var Log = logging.WithName(loggerName)

func Configure() error {
	if klog.V(1).Enabled() {
		return logging.Setup(logging.TextFormat, 0)
	}
	return nil
}
