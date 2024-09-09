package event

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Info defines the event details
type Info struct {
	Regarding corev1.ObjectReference
	Related   *corev1.ObjectReference
	Reason    Reason
	Message   string
	Action    Action
	Source    Source
	Type      string
}

func (i *Info) Resource() string {
	if i.Regarding.Namespace == "" {
		return strings.Join([]string{i.Regarding.Kind, i.Regarding.Name}, "/")
	}
	return strings.Join([]string{i.Regarding.Kind, i.Regarding.Namespace, i.Regarding.Name}, "/")
}
