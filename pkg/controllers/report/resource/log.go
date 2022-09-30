package resource

import "sigs.k8s.io/controller-runtime/pkg/log"

const controllerName = "resource-report-controller"

var logger = log.Log.WithName(controllerName)
