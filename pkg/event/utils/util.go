package utils

const EventSource = "policy-controller"

const EventWorkQueueName = "policy-controller-events"

type EventInfo struct {
	Kind     string
	Resource string
	Rule     string
	Reason   string
	Message  string
}

const EventWorkerThreadCount = 1
