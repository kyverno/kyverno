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
	"github.com/kyverno/kyverno/pkg/engine/apicall"
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
				APICall:         kyvernov1.APICall{URLPath: "/apis", Method: "GET"},
				RefreshInterval: &metav1.Duration{Duration: time.Hour},
				RetryLimit:      1,
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

// Verify the real New/Stop shutdown path. Stop cancels requests promptly,
// but waits until a callback already inside the publication gate has exited.
func TestEntry_Publish_SerializesWithStop(t *testing.T) {
	gce := reliabilityTestGCE()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	e, err := New(ctx, gce, &eventCollector{events: make(chan event.Info, 2)},
		nil, nil, logr.Discard(), &blockingAPIClient{started: make(chan struct{})},
		gce.Spec.APICall.APICall, time.Hour, 0, time.Second, false, nil)
	require.NoError(t, err)

	entry := e.(*entry)
	started := make(chan struct{})
	release := make(chan struct{})
	published := make(chan struct{})
	go func() {
		defer close(published)
		entry.publish(ctx, func() {
			close(started)
			<-release
			entry.setData([]byte(`{"phase":"initial"}`), nil)
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not start")
	}

	stopStarted := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		close(stopStarted)
		e.Stop()
		close(stopped)
	}()
	// Synchronize the attempt before asserting that the in-progress
	// publication prevents Stop from completing.
	select {
	case <-stopStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not start")
	}
	select {
	case <-stopped:
		t.Fatal("Stop completed while publication was blocked")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("publication blocked")
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop blocked")
	}

	called := false
	entry.publish(ctx, func() { called = true })
	require.False(t, called, "a late poll must not publish after Stop")
	result, err := entry.Get("")
	require.NoError(t, err)
	require.Equal(t, "initial", result.(map[string]any)["phase"])
}

type cancelingExecutor struct {
	cancel context.CancelFunc
	calls  int
	err    error
}

func (c *cancelingExecutor) Execute(context.Context, *kyvernov1.APICall) ([]byte, error) {
	c.calls++
	c.cancel()
	return nil, c.err
}

// A canceled context must not trigger additional retries after a backend error.
func TestDoCall_CancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backendErr := errors.New("backend failed during cancellation")
	caller := &cancelingExecutor{cancel: cancel, err: backendErr}
	_, err := doCall(ctx, caller, kyvernov1.APICall{}, 3)
	require.ErrorIs(t, err, backendErr)
	require.Equal(t, 1, caller.calls)
}

var _ apicall.Executor = (*cancelingExecutor)(nil)

// A cancelled parent context must also suppress publication without Stop.
func TestEntry_Publish_RejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &entry{}
	called := false
	e.publish(ctx, func() { called = true })
	require.False(t, called)
}
