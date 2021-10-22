package event

const eventWorkQueueName = "kyverno-events"

const workQueueRetryLimit = 10

const eventsWorkerUID string = "events-worker"

//Info defines the event details
type Info struct {
	Kind      string
	Name      string
	Namespace string
	Reason    string
	Message   string
	Source    Source
}
