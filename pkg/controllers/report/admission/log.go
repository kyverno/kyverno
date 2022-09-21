package admission

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "admission-report-controller"
	logger         = log.Log.WithName(controllerName)
)
