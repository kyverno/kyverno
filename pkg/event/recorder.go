package event

import (
	"context"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	typedeventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
)

func newSink(sink typedeventsv1.EventsV1Interface) events.EventSink {
	return &events.EventSinkImpl{
		Interface: sink,
	}
}

func newBroadcaster(sink events.EventSink) events.EventBroadcaster {
	return events.NewBroadcaster(sink)
}

func startRecording(ctx context.Context, broadcaster events.EventBroadcaster, source Source) (events.EventRecorder, error) {
	logger := klog.Background().V(int(0))
	if err := broadcaster.StartRecordingToSinkWithContext(ctx); err != nil {
		return nil, err
	}
	// TODO: we should probably wait workers exited before stopping recorders
	if _, err := broadcaster.StartLogging(logger); err != nil {
		return nil, err
	}
	return broadcaster.NewRecorder(scheme.Scheme, string(source)), nil
}
