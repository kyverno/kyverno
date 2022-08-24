package config

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "config-controller"
	logger         = log.Log.WithName(controllerName)
)
