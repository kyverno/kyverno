package aggregate

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "aggregate-report-controller"
	logger         = log.Log.WithName(controllerName)
)
