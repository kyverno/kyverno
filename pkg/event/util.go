package event

const eventSource = "policy-controller"

const eventWorkQueueName = "policy-controller-events"

const eventWorkerThreadCount = 1

const workQueueRetryLimit = 5

//Info defines the event details
type Info struct {
	Kind      string
	Name      string
	Namespace string
	Reason    string
	Message   string
}
