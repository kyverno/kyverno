package event

import (
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
)

type eventKey struct {
	eventType           string
	action              string
	reason              string
	reportingController string
	reportingInstance   string
	regarding           corev1.ObjectReference
	related             corev1.ObjectReference
}

type EventCache map[eventKey]*eventsv1.Event

func getKey(event *eventsv1.Event) eventKey {
	key := eventKey{
		eventType:           event.Type,
		action:              event.Action,
		reason:              event.Reason,
		reportingController: event.ReportingController,
		reportingInstance:   event.ReportingInstance,
		regarding:           event.Regarding,
	}
	if event.Related != nil {
		key.related = *event.Related
	}
	return key
}
