package event

const eventWorkQueueName = "kyverno-events"

const eventWorkerThreadCount = 1

const workQueueRetryLimit = 5

//Info defines the event details
type Info struct {
	Kind      string
	Name      string
	Namespace string
	Reason    string
	Message   string
	Source    Source
}
