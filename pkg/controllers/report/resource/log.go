package resource

import "github.com/kyverno/kyverno/pkg/logging"

// var logger = logging.ControllerLogger(ControllerName)
var logger = logging.WithName(ControllerName)
