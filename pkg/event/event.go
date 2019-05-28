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

type Events []Event

func (e *Event) String() string {
	message := fmt.Sprintf("%s: For policy %s, for object with UID %s:\n", e.Reason.String(), e.Policy, e.ObjectUID)
	for _, m := range e.Messages {
		message += fmt.Sprintf("    * %s\n", m)
	}

	// remove last line feed
	if 0 != len(message) {
		message = message[:len(message)-1]
	}
	return message
}

func (e *Events) String() string {
	message := ""
	for _, event := range *e {
		message += (event.String() + "\n")
	}

	// remove last line feed
	if 0 != len(message) {
		message = message[:len(message)-1]
	}

	return message
}
