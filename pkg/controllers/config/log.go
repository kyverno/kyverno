package config

import "github.com/kyverno/kyverno/pkg/logging"

const controllerName = "config-controller"

var logger = logging.WithName(controllerName)
