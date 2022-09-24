package admission

import "sigs.k8s.io/controller-runtime/pkg/log"

const controllerName = "admission-report-controller"

var logger = log.Log.WithName(controllerName)
