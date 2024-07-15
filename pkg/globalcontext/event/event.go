package event

import (
	"github.com/kyverno/kyverno/pkg/event"
	corev1 "k8s.io/api/core/v1"
)

const (
	source = "globalcontext-controller"
	action = "Retrying"
)

func NewErrorEvent(regarding corev1.ObjectReference, err error) event.Info {
	return event.Info{
		Regarding: regarding,
		Source:    source,
		Reason:    event.PolicyError,
		Message:   err.Error(),
		Action:    action,
		Type:      corev1.EventTypeWarning,
	}
}
