package cleanup

import (
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

func newRecorder(client dclient.Interface) record.EventRecorder {
	utilruntime.Must(scheme.AddToScheme(scheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: client.GetEventsInterface()})
	return eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "cleanup-controller"})
}
