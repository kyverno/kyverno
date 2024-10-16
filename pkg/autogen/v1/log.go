package v1

import "github.com/kyverno/kyverno/pkg/logging"

var (
	logger = logging.WithName("autogen-v1")
	debug  = logger.V(5)
)
