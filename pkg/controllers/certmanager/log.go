package certmanager

import "sigs.k8s.io/controller-runtime/pkg/log"

const controllerName = "certmanager-controller"

var logger = log.Log.WithName(controllerName)
