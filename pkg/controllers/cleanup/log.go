package cleanup

import "sigs.k8s.io/controller-runtime/pkg/log"

const ControllerName = "cleanup-controller"

var logger = log.Log.WithName(ControllerName)
