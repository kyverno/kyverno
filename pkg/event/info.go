package event

// Info defines the event details
type Info struct {
	Kind      string
	Name      string
	Namespace string
	Reason    Reason
	Message   string
	Source    Source
}
