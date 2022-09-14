package report

import "sigs.k8s.io/controller-runtime/pkg/log"

var (
	controllerName = "report-controller"
	logger         = log.Log.WithName(controllerName)
)
