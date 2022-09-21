package background

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "background-scan-controller"
	logger         = log.Log.WithName(controllerName)
)
