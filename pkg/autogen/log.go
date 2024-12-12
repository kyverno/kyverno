package autogen

import "github.com/kyverno/kyverno/pkg/logging"

var (
	logger = logging.WithName("autogen")
	debug  = logger.V(5)
)
