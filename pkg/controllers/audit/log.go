package audit

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "audit-controller"
	logger         = log.Log.WithName(controllerName)
)
