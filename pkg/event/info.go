package event

import "strings"

// Info defines the event details
type Info struct {
	Kind      string
	Name      string
	Namespace string
	Reason    Reason
	Message   string
	Source    Source
}

func (i *Info) Resource() string {
	if i.Namespace == "" {
		return strings.Join([]string{i.Kind, i.Name}, "/")
	}
	return strings.Join([]string{i.Kind, i.Namespace, i.Name}, "/")
}
