package externalapi

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	typedv2beta1 "github.com/kyverno/kyverno/pkg/client/clientset/versioned/typed/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/apicall"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
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
				APICall:         kyverno.APICall{URLPath: "/apis", Method: "GET"},
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
		nil, nil, logr.Discard(), immediateAPIClient{},
		gce.Spec.APICall.APICall, time.Hour, 0, time.Second, false, nil)
	require.NoError(t, err)

	entry := e.(*entry)
	// Drain the initial successful poll before blocking publication ourselves.
	// The worker is then sleeping until the next (hourly) tick and cannot
	// contend for publishMu or hide a missing shutdown gate.
	require.Eventually(t, func() bool {
		_, getErr := entry.Get("")
		return getErr == nil
	}, 5*time.Second, time.Millisecond, "initial poll did not publish")
	// Get succeeding proves setData ran, but the poll may still hold
	// publishMu while finishing its publication callback. Acquiring it
	// here waits for that callback to exit before this test owns the gate.
	entry.publishMu.Lock()
	entry.publishMu.Unlock()

	started := make(chan struct{})
	release := make(chan struct{})
	published := make(chan struct{})
	go func() {
		defer close(published)
		entry.publish(ctx, func() {
			close(started)
			<-release
			entry.setData(ctx, []byte(`{"phase":"initial"}`), nil)
		})
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("publication did not start")
	}

	stopped := make(chan struct{})
	go func() {
		e.Stop()
		close(stopped)
	}()

	// This probe is valid only because New was called with
	// shouldUpdateStatus=false (and nil kyvernoClient), so Stop is the
	// only goroutine that can hold statusMu. A successful TryLock releases
	// it immediately; a failed TryLock means Stop has passed cancellation
	// and is waiting on publishMu, still held by the callback above.
	require.Eventually(t, func() bool {
		if entry.statusMu.TryLock() {
			entry.statusMu.Unlock()
			return false
		}
		return true
	}, 5*time.Second, time.Millisecond, "Stop did not reach the publication gate")
	select {
	case <-stopped:
		t.Fatal("Stop completed before the in-flight publication exited")
	default:
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

func (c *cancelingExecutor) Execute(context.Context, *kyverno.APICall) ([]byte, error) {
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
	_, err := doCall(ctx, caller, kyverno.APICall{}, 3)
	require.ErrorIs(t, err, backendErr)
	require.Equal(t, 1, caller.calls)
}

var _ apicall.Executor = (*cancelingExecutor)(nil)

// backoffContext signals after the retry decision observed an active context.
type backoffContext struct {
	context.Context
	once     sync.Once
	executed atomic.Bool
	waiting  chan struct{}
}

func (c *backoffContext) Err() error {
	err := c.Context.Err()
	if err == nil && c.executed.Load() {
		c.once.Do(func() { close(c.waiting) })
	}
	return err
}

type failingExecutor struct {
	calls     int
	err       error
	onExecute func()
}

func (c *failingExecutor) Execute(context.Context, *kyverno.APICall) ([]byte, error) {
	c.calls++
	if c.onExecute != nil {
		c.onExecute()
	}
	return nil, c.err
}

func TestDoCall_CancellationInterruptsRetryBackoff(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &backoffContext{Context: parent, waiting: make(chan struct{})}
	caller := &failingExecutor{err: errors.New("backend unavailable"), onExecute: func() { ctx.executed.Store(true) }}
	result := make(chan error, 1)
	go func() {
		_, err := doCallWithBackoff(ctx, caller, kyverno.APICall{}, wait.Backoff{Duration: 10 * time.Second, Steps: 3})
		result <- err
	}()

	select {
	case <-ctx.waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("retry decision did not observe an active context")
	}
	cancel()
	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
		require.Equal(t, 1, caller.calls)
	case <-time.After(time.Second):
		t.Fatal("retry backoff did not stop promptly after cancellation")
	}
}

func TestDoCall_RetryExhaustionReturnsBackendError(t *testing.T) {
	backendErr := errors.New("backend unavailable")
	caller := &failingExecutor{err: backendErr}
	_, err := doCallWithBackoff(context.Background(), caller, kyverno.APICall{}, wait.Backoff{Steps: 3})
	require.ErrorIs(t, err, backendErr)
	require.Equal(t, 3, caller.calls)
}

type recoveringExecutor struct {
	calls int
}

func (c *recoveringExecutor) Execute(context.Context, *kyverno.APICall) ([]byte, error) {
	c.calls++
	if c.calls == 1 {
		return nil, errors.New("temporary backend failure")
	}
	return []byte(`{"ok":true}`), nil
}

func TestDoCall_RetriesUntilSuccess(t *testing.T) {
	caller := &recoveringExecutor{}
	result, err := doCallWithBackoff(context.Background(), caller, kyverno.APICall{}, wait.Backoff{Steps: 3})
	require.NoError(t, err)
	require.Equal(t, []byte(`{"ok":true}`), result)
	require.Equal(t, 2, caller.calls)
}

// A cancelled parent context must also suppress publication without Stop.
func TestEntry_Publish_RejectsCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &entry{}
	called := false
	e.publish(ctx, func() { called = true })
	require.False(t, called)
}

// immediateAPIClient lets the polling worker reach a real status refresh
// without depending on network traffic or a timing-sensitive HTTP server.
type immediateAPIClient struct{}

func (immediateAPIClient) RawAbsPath(context.Context, string, string, io.Reader) ([]byte, error) {
	return []byte(`{"ok":true}`), nil
}

// The test client preserves the generated interfaces but blocks exactly the
// Kubernetes Get invoked by updateStatus. It never needs a live API server.
type blockingStatusClient struct {
	versioned.Interface
	started  chan struct{}
	canceled chan struct{}
}

func (c *blockingStatusClient) KyvernoV2beta1() typedv2beta1.KyvernoV2beta1Interface {
	return &blockingStatusV2beta1{client: c}
}

type blockingStatusV2beta1 struct {
	typedv2beta1.KyvernoV2beta1Interface
	client *blockingStatusClient
}

func (c *blockingStatusV2beta1) GlobalContextEntries() typedv2beta1.GlobalContextEntryInterface {
	return &blockingStatusEntries{client: c.client}
}

type blockingStatusEntries struct {
	typedv2beta1.GlobalContextEntryInterface
	client *blockingStatusClient
}

func (c *blockingStatusEntries) Get(ctx context.Context, _ string, _ metav1.GetOptions) (*kyvernov2beta1.GlobalContextEntry, error) {
	close(c.client.started)
	<-ctx.Done()
	close(c.client.canceled)
	return nil, ctx.Err()
}

// Regression: Stop must cancel a Kubernetes status Get before waiting on
// statusMu. Reversing that order would deadlock this production New/Stop path.
func TestEntry_Stop_CancelsInflightStatusRefresh(t *testing.T) {
	gce := reliabilityTestGCE()
	client := &blockingStatusClient{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	e, err := New(context.Background(), gce, event.NewFake(), client, nil, logr.Discard(),
		immediateAPIClient{}, gce.Spec.APICall.APICall, time.Hour, 0, time.Second, true, nil)
	require.NoError(t, err)

	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		e.Stop()
		t.Fatal("status refresh did not reach the Kubernetes Get")
	}

	stopped := make(chan struct{})
	go func() {
		e.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop deadlocked with an in-flight Kubernetes status Get")
	}
	select {
	case <-client.canceled:
	default:
		t.Fatal("status Get did not observe Stop cancellation")
	}
}

// Status refresh must not invoke the Kubernetes client after cancellation.
func TestEntry_RefreshStatus_SkipsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	e := &entry{}
	e.refreshStatus(ctx, reliabilityTestGCE(), nil, logr.Discard())
}
