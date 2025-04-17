package v2

import "github.com/kyverno/kyverno/pkg/logging"

var (
	logger = logging.WithName("autogen-v2")
	debug  = logger.V(5)
)
