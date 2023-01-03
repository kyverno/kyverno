package openapi

import "k8s.io/apimachinery/pkg/util/runtime"

func overrideRuntimeErrorHandler() {
	if len(runtime.ErrorHandlers) > 0 {
		runtime.ErrorHandlers[0] = func(err error) {
			logger.V(6).Info("runtime error", "msg", err.Error())
		}
	} else {
		runtime.ErrorHandlers = []func(err error){
			func(err error) {
				logger.V(6).Info("runtime error", "msg", err.Error())
			},
		}
	}
}
