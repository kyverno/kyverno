package externalapi

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// eventCollector records the actual events emitted by the polling callback;
// event.NewFake() discards them and cannot verify this regression.
type eventCollector struct {
	events chan event.Info
}

func (c *eventCollector) Add(infos ...event.Info) {
	for _, info := range infos {
		select {
		case c.events <- info:
		default:
		}
	}
}

type failedAPIClient struct{}

func (failedAPIClient) RawAbsPath(context.Context, string, string, io.Reader) ([]byte, error) {
	return nil, errors.New("backend unavailable")
}

func reliabilityTestGCE() *kyvernov2beta1.GlobalContextEntry {
	return &kyvernov2beta1.GlobalContextEntry{
		ObjectMeta: metav1.ObjectMeta{Name: "gce-reliability"},
		Spec: kyvernov2beta1.GlobalContextEntrySpec{
			APICall: &kyvernov2beta1.ExternalAPICall{
				APICall: kyvernov1.APICall{URLPath: "/apis", Method: "GET"},
				RefreshInterval: &metav1.Duration{Duration: time.Hour},
				RetryLimit: 1,
			},
		},
	}
}

func TestEntry_Stop_DoesNotEmitCancellationEvents(t *testing.T) {
	client := &blockingAPIClient{started: make(chan struct{})}
	recorder := &eventCollector{events: make(chan event.Info, 2)}
	gce := reliabilityTestGCE()

	e, err := New(context.Background(), gce, recorder, nil, nil, logr.Discard(),
		client, gce.Spec.APICall.APICall, time.Hour, 0, time.Second, false, nil)
	require.NoError(t, err)

	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		e.Stop()
		t.Fatal("API call was not started")
	}
	e.Stop() // Waits for the in-flight call and poll worker.
	select {
	case <-recorder.events:
		t.Fatal("normal entry teardown emitted a Warning event")
	default:
	}
}

// Genuine API failures must still emit error events: cancellation suppression
// must not accidentally silence actual backend failures.
func TestEntry_APIErrorStillEmitsEvent(t *testing.T) {
	recorder := &eventCollector{events: make(chan event.Info, 2)}
	gce := reliabilityTestGCE()

	e, err := New(context.Background(), gce, recorder, nil, nil, logr.Discard(),
		failedAPIClient{}, gce.Spec.APICall.APICall, time.Hour, 0, time.Second, false, nil)
	require.NoError(t, err)
	defer e.Stop()

	select {
	case <-recorder.events:
	case <-time.After(5 * time.Second):
		t.Fatal("a genuine API failure did not emit an error event")
	}
}
