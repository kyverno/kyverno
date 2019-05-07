package interfaces

import (
	controllerinterfaces "github.com/nirmata/kube-policy/controller/interfaces"
	utils "github.com/nirmata/kube-policy/pkg/event/utils"
)

type BuilderInternal interface {
	SetController(controller controllerinterfaces.PolicyGetter)
	Run(threadiness int, stopCh <-chan struct{}) error
	AddEvent(info utils.EventInfo)
}
