package background

import "sigs.k8s.io/controller-runtime/pkg/log"

const controllerName = "background-scan-controller"

var logger = log.Log.WithName(controllerName)
