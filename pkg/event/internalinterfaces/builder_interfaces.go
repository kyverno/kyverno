package internalinterfaces

import (
	internalinterfaces "github.com/nirmata/kube-policy/controller/internalinterfaces"
	utils "github.com/nirmata/kube-policy/pkg/event/utils"
)

type BuilderInternal interface {
	SetController(controller internalinterfaces.PolicyGetter)
	Run(threadiness int, stopCh <-chan struct{}) error
	AddEvent(info utils.EventInfo)
}
