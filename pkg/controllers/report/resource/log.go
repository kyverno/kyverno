package resource

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "resource-report-controller"
	logger         = log.Log.WithName(controllerName)
)
