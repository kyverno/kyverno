package background

import "sigs.k8s.io/controller-runtime/pkg/log"

const controllerName = "webhook-ca-controller"

var logger = log.Log.WithName(controllerName)
