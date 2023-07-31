package event

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	typedeventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/tools/events"
)

func NewRecorder(source Source, sink typedeventsv1.EventsV1Interface) events.EventRecorder {
	utilruntime.Must(scheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := events.NewBroadcaster(
		&events.EventSinkImpl{
			Interface: sink,
		},
	)
	eventBroadcaster.StartStructuredLogging(0)
	stopCh := make(chan struct{})
	eventBroadcaster.StartRecordingToSink(stopCh)
	return eventBroadcaster.NewRecorder(scheme.Scheme, string(source))
}
