package event

import "fmt"

type Event struct {
	Policy string
	// We can extract only UID and GVK from admission request
	// Kubernetes will fix this in future
	ObjectUID string
	Reason    Reason
	Messages  []string
}

func (e *Event) String() string {
	message := fmt.Sprintf("%s: For policy %s, for object with UID %s:\n", e.Reason.String(), e.Policy, e.ObjectUID)
	for _, m := range e.Messages {
		message += fmt.Sprintf("    * %s\n", m)
	}

	// remove last line feed
	message = message[:len(message)-1]
	return message
}
