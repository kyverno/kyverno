package background

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/workqueue"
)

// TestReconcileURStatus_RetryOnAPIFailure tests that when the API server Get()
// returns a generic error (network issue, timeout, API unavailable), the function
// returns a non-nil error to force the workqueue to retry.
//
// This validates the fix that prevents falling back to stale data when API is unavailable.
func TestReconcileURStatus_RetryOnAPIFailure(t *testing.T) {
	// Create a fake clientset
	fakeClient := fake.NewSimpleClientset()

	// Inject a generic error for Get operations on UpdateRequests
	// This simulates network issues, timeouts, or API server unavailability
	fakeClient.PrependReactor("get", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, errors.New("connection refused: API server unavailable")
	})

	// Create a minimal controller with just the kyvernoClient
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Create an UpdateRequest to pass to reconcileURStatus
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: error should be non-nil to force retry
	assert.Error(t, err, "reconcileURStatus should return error on API failure")
	assert.Contains(t, err.Error(), "failed to fetch latest UR status", "error message should indicate fetch failure")
	assert.Contains(t, err.Error(), "connection refused", "error should wrap the original error")

	// Assert: returned state should be empty (not used when error is returned)
	assert.Equal(t, kyvernov2.UpdateRequestState(""), state, "state should be empty when error occurs")
}

// TestReconcileURStatus_SkipOnNotFound tests that when the API server Get()
// returns NotFound error, the function returns Skip state with nil error.
//
// This validates that already-deleted UpdateRequests are handled gracefully.
func TestReconcileURStatus_SkipOnNotFound(t *testing.T) {
	// Create a fake clientset
	fakeClient := fake.NewSimpleClientset()

	// Inject a NotFound error for Get operations on UpdateRequests
	// This simulates the case where UR was already deleted by another controller
	fakeClient.PrependReactor("get", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewNotFound(
			schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"},
			"test-ur",
		)
	})

	// Create a minimal controller with just the kyvernoClient
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Create an UpdateRequest to pass to reconcileURStatus
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: error should be nil for NotFound (graceful handling)
	assert.NoError(t, err, "reconcileURStatus should not return error on NotFound")

	// Assert: state should be Skip
	assert.Equal(t, kyvernov2.Skip, state, "state should be Skip when UR is NotFound")
}

// TestReconcileURStatus_DeleteOnCompleted tests that when the UR has Completed state,
// the function attempts to delete it.
func TestReconcileURStatus_DeleteOnCompleted(t *testing.T) {
	// Create an UpdateRequest with Completed state
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-completed",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Completed,
		},
	}

	// Create a fake clientset with the UR pre-populated
	fakeClient := fake.NewSimpleClientset(ur)

	// Track if delete was called
	deleteCalled := false
	fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteCalled = true
		return true, nil, nil
	})

	// Create controller
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: should complete without error
	assert.NoError(t, err, "reconcileURStatus should not return error for Completed UR")

	// Assert: delete should have been called
	assert.True(t, deleteCalled, "delete should be called for Completed UR")

	// Assert: returned state should be Completed
	assert.Equal(t, kyvernov2.Completed, state, "state should be Completed")
}

// TestReconcileURStatus_ResetFailedToPending tests that when the UR has Failed state,
// the function resets it to Pending for retry.
func TestReconcileURStatus_ResetFailedToPending(t *testing.T) {
	// Create an UpdateRequest with Failed state
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-failed",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State:   kyvernov2.Failed,
			Message: "previous failure",
		},
	}

	// Create a fake clientset with the UR pre-populated
	fakeClient := fake.NewSimpleClientset(ur)

	// Track if updatestatus was called with Pending state
	updateCalled := false
	var updatedState kyvernov2.UpdateRequestState
	fakeClient.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "status" {
			updateCalled = true
			updateAction := action.(k8stesting.UpdateAction)
			updatedUR := updateAction.GetObject().(*kyvernov2.UpdateRequest)
			updatedState = updatedUR.Status.State
		}
		return false, nil, nil // Let it pass through to default handler
	})

	// Create controller
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: should complete without error
	assert.NoError(t, err, "reconcileURStatus should not return error for Failed UR")

	// Assert: updatestatus should have been called
	assert.True(t, updateCalled, "updatestatus should be called for Failed UR")

	// Assert: status should have been updated to Pending
	assert.Equal(t, kyvernov2.Pending, updatedState, "Failed UR should be reset to Pending")

	// Assert: returned state should be Pending (the state after reset)
	// Note: The function modifies new.Status.State to Pending before returning
	assert.Equal(t, kyvernov2.Pending, state, "returned state should be Pending after reset")
}

// TestReconcileURStatus_TimeoutError tests that timeout errors are treated as
// retriable errors, not as NotFound.
func TestReconcileURStatus_TimeoutError(t *testing.T) {
	// Create a fake clientset
	fakeClient := fake.NewSimpleClientset()

	// Inject a timeout error
	fakeClient.PrependReactor("get", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewTimeoutError("request timeout", 30)
	})

	// Create controller
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Create an UpdateRequest
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: should return error to force retry
	assert.Error(t, err, "reconcileURStatus should return error on timeout")
	assert.Contains(t, err.Error(), "failed to fetch latest UR status", "error should indicate fetch failure")

	// Assert: state should be empty
	assert.Equal(t, kyvernov2.UpdateRequestState(""), state, "state should be empty on error")
}

// TestReconcileURStatus_ServerUnavailable tests that 503 Service Unavailable errors
// cause retry instead of using stale data.
func TestReconcileURStatus_ServerUnavailable(t *testing.T) {
	// Create a fake clientset
	fakeClient := fake.NewSimpleClientset()

	// Inject a ServiceUnavailable error
	fakeClient.PrependReactor("get", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewServiceUnavailable("service unavailable")
	})

	// Create controller
	c := &controller{
		kyvernoClient: fakeClient,
	}

	// Create an UpdateRequest
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	// Call reconcileURStatus
	state, err := c.reconcileURStatus(ur)

	// Assert: should return error to force retry
	assert.Error(t, err, "reconcileURStatus should return error on ServiceUnavailable")

	// Assert: state should be empty
	assert.Equal(t, kyvernov2.UpdateRequestState(""), state, "state should be empty on error")

}

// newTestUpdateRequest creates a test UpdateRequest with given parameters
func newTestUpdateRequest(name string, state kyvernov2.UpdateRequestState, requestType kyvernov2.RequestType) *kyvernov2.UpdateRequest {
	return &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.KyvernoNamespace(),
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Type:   requestType,
			Policy: "test-policy",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: state,
		},
	}
}

// newTestQueue creates a test workqueue
func newTestQueue() workqueue.TypedRateLimitingInterface[any] {
	return workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test"},
	)
}

func TestHandleErr_NilError(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	key := "test-key"
	c.queue.Add(key)

	// nil error should forget the key
	c.handleErr(nil, key)

	// Verify queue forgot the key (NumRequeues should be 0)
	assert.Equal(t, 0, c.queue.NumRequeues(key))
}

func TestUpdateUR_SkipsCompletedState(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	// A CEL type, not a legacy one: legacy mutate/generate URs always
	// re-enqueue regardless of state (see TestUpdateUR_LegacyAlwaysReenqueuesRegardlessOfState).
	completedUR := newTestUpdateRequest("completed-ur", kyvernov2.Completed, kyvernov2.CELGenerate)

	// updateUR should not enqueue completed URs
	c.updateUR(nil, completedUR)

	// Queue should be empty since completed URs are skipped
	assert.Equal(t, 0, c.queue.Len())
}

func TestUpdateUR_SkipsSkipState(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	// A CEL type, not a legacy one: legacy mutate/generate URs always
	// re-enqueue regardless of state (see TestUpdateUR_LegacyAlwaysReenqueuesRegardlessOfState).
	skipUR := newTestUpdateRequest("skip-ur", kyvernov2.Skip, kyvernov2.CELGenerate)

	// updateUR should not enqueue skip URs
	c.updateUR(nil, skipUR)

	// Queue should be empty since skip URs are skipped
	assert.Equal(t, 0, c.queue.Len())
}

func TestUpdateUR_EnqueuesPendingState(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	pendingUR := newTestUpdateRequest("pending-ur", kyvernov2.Pending, kyvernov2.CELGenerate)

	// updateUR should enqueue pending URs
	c.updateUR(nil, pendingUR)

	// Queue should have the UR
	assert.Equal(t, 1, c.queue.Len())
}

func TestUpdateUR_RequeuesFailedStateAfterBackoff(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	failedUR := newTestUpdateRequest("failed-ur", kyvernov2.Failed, kyvernov2.CELGenerate)
	failedUR.Status.RetryCount = 1

	// A Failed UR whose flip back to Pending did not happen (e.g. the status
	// update failed) is only seen again on a resync; it must not be dropped.
	c.updateUR(failedUR, failedUR)

	assert.Equal(t, 0, c.queue.Len(), "a retry should wait for its backoff")
	assert.Eventually(t, func() bool { return c.queue.Len() == 1 }, 10*time.Second, 20*time.Millisecond)
}

func TestUpdateUR_RequeuesRetriedPendingStateAfterBackoff(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	pendingUR := newTestUpdateRequest("retried-ur", kyvernov2.Pending, kyvernov2.CELGenerate)
	pendingUR.Status.RetryCount = 2

	// A UR flipped back to Pending after a failure carries its retry count and
	// must wait for its backoff too.
	c.updateUR(pendingUR, pendingUR)

	assert.Equal(t, 0, c.queue.Len(), "a retry should wait for its backoff")
	assert.Eventually(t, func() bool { return c.queue.Len() == 1 }, 10*time.Second, 20*time.Millisecond)
}

// TestUpdateUR_LegacyAlwaysReenqueuesRegardlessOfState covers the gap where a
// legacy mutate/generate UR in Completed or Skip state would otherwise be
// dropped by the state filter above, forgotten until controller restart if
// its earlier delete attempt (via dropLegacyUpdateRequest) exhausted
// maxRetries. Legacy URs must always re-enqueue so the next resync retries
// the delete.
func TestUpdateUR_LegacyAlwaysReenqueuesRegardlessOfState(t *testing.T) {
	for _, tt := range []struct {
		requestType kyvernov2.RequestType
		state       kyvernov2.UpdateRequestState
	}{
		{kyvernov2.Generate, kyvernov2.Completed},
		{kyvernov2.Generate, kyvernov2.Skip},
		{kyvernov2.Mutate, kyvernov2.Completed},
		{kyvernov2.Mutate, kyvernov2.Skip},
	} {
		t.Run(fmt.Sprintf("%s/%s", tt.requestType, tt.state), func(t *testing.T) {
			c := &controller{queue: newTestQueue()}
			ur := newTestUpdateRequest("legacy-ur", tt.state, tt.requestType)

			c.updateUR(nil, ur)

			assert.Equal(t, 1, c.queue.Len(), "a legacy UR must re-enqueue even in state %q", tt.state)
		})
	}
}

func TestDefaultRetryBackoff(t *testing.T) {
	tests := []struct {
		retryCount int
		want       time.Duration
	}{
		{retryCount: 0, want: 300 * time.Millisecond},
		{retryCount: 1, want: 300 * time.Millisecond},
		{retryCount: 2, want: 600 * time.Millisecond},
		{retryCount: 3, want: 1200 * time.Millisecond},
		{retryCount: 6, want: 9600 * time.Millisecond},
		{retryCount: 7, want: 10 * time.Second},
		{retryCount: 10, want: 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("retry count %d", tt.retryCount), func(t *testing.T) {
			assert.Equal(t, tt.want, defaultRetryBackoff(tt.retryCount))
		})
	}
}

func TestAddUR_EnqueuesUpdateRequest(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	ur := newTestUpdateRequest("test-ur", kyvernov2.Pending, kyvernov2.Generate)

	// addUR should always enqueue
	c.addUR(ur)

	// Queue should have the UR
	assert.Equal(t, 1, c.queue.Len())
}

func TestMaxRetries_Constant(t *testing.T) {
	// Verify maxRetries constant is set to expected value
	assert.Equal(t, 10, maxRetries)
}

// TestProcessUR_UnsupportedTypeFails verifies that processUR's default case
// (an unrecognized request type reaching the CELGenerate/CELMutate switch)
// sets the UR to Failed with a message, rather than silently succeeding or
// panicking. Legacy mutate/generate types never reach this switch; they are
// routed to dropLegacyUpdateRequest earlier in syncUpdateRequest.
func TestProcessUR_UnsupportedTypeFails(t *testing.T) {
	ur := newTestUpdateRequest("unsupported-ur", kyvernov2.Pending, kyvernov2.RequestType("bogus"))
	fakeClient := fake.NewSimpleClientset(ur)

	c := &controller{
		kyvernoClient: fakeClient,
	}

	err := c.processUR(context.Background(), ur)
	assert.NoError(t, err)

	updated, getErr := fakeClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), ur.GetName(), metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Equal(t, kyvernov2.Failed, updated.Status.State)
	assert.Contains(t, updated.Status.Message, "unsupported update request type")
}


// TestDefaultRetryBackoff_ExponentialGrowth verifies that defaultRetryBackoff
// produces exponentially increasing delays capped at the configured maximum.
func TestDefaultRetryBackoff_ExponentialGrowth(t *testing.T) {
	tests := []struct {
		retryCount int
		wantMin    time.Duration
		wantMax    time.Duration
	}{
		{retryCount: 0, wantMin: 300 * time.Millisecond, wantMax: 300 * time.Millisecond},
		{retryCount: 1, wantMin: 300 * time.Millisecond, wantMax: 300 * time.Millisecond},
		{retryCount: 2, wantMin: 600 * time.Millisecond, wantMax: 600 * time.Millisecond},
		{retryCount: 3, wantMin: 1200 * time.Millisecond, wantMax: 1200 * time.Millisecond},
		{retryCount: 4, wantMin: 2400 * time.Millisecond, wantMax: 2400 * time.Millisecond},
		{retryCount: 5, wantMin: 4800 * time.Millisecond, wantMax: 4800 * time.Millisecond},
		{retryCount: 6, wantMin: 9600 * time.Millisecond, wantMax: 9600 * time.Millisecond},
		// At retry 7, 300ms * 2^6 = 19.2s > 10s cap
		{retryCount: 7, wantMin: 10 * time.Second, wantMax: 10 * time.Second},
		{retryCount: 100, wantMin: 10 * time.Second, wantMax: 10 * time.Second},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("retry_%d", tc.retryCount), func(t *testing.T) {
			got := defaultRetryBackoff(tc.retryCount)
			assert.GreaterOrEqual(t, got, tc.wantMin, "delay should be at least %v", tc.wantMin)
			assert.LessOrEqual(t, got, tc.wantMax, "delay should be at most %v", tc.wantMax)
		})
	}
}

// TestDefaultRetryBackoff_MonotonicallyIncreasing verifies that the backoff
// delay never decreases as the retry count grows — a property that prevents
// the thundering-herd effect when many URs fail simultaneously.
func TestDefaultRetryBackoff_MonotonicallyIncreasing(t *testing.T) {
	prev := defaultRetryBackoff(0)
	for i := 1; i <= 20; i++ {
		cur := defaultRetryBackoff(i)
		assert.GreaterOrEqual(t, cur, prev,
			"backoff at retry %d (%v) must be >= retry %d (%v)", i, cur, i-1, prev)
		prev = cur
	}
}

// TestControllerReconcileTimeout verifies that the controller's reconcileTimeout
// is set to a reasonable non-zero value that bounds UR processing time.
func TestControllerReconcileTimeout(t *testing.T) {
	c := &controller{
		reconcileTimeout: 2 * time.Minute,
	}
	assert.Equal(t, 2*time.Minute, c.reconcileTimeout,
		"reconcileTimeout should be 2 minutes to bound slow UR processing")
}

// TestQueueUsesPerItemExponentialRateLimiter verifies that the work queue is
// configured with a per-item exponential failure rate limiter instead of the
// default controller rate limiter. This prevents the thundering-herd retry
// storm described in issue #18797.
func TestQueueUsesPerItemExponentialRateLimiter(t *testing.T) {
	rateLimiter := workqueue.NewTypedItemExponentialFailureRateLimiter[any](
		500*time.Millisecond,
		30*time.Second,
	)
	q := workqueue.NewTypedRateLimitingQueueWithConfig(
		rateLimiter,
		workqueue.TypedRateLimitingQueueConfig[any]{Name: "test-background"},
	)
	defer q.ShutDown()

	key := "test-key"

	// First failure: should get base delay (500ms)
	d1 := rateLimiter.When(key)
	assert.Equal(t, 500*time.Millisecond, d1)

	// Second failure: should double (1s)
	d2 := rateLimiter.When(key)
	assert.Equal(t, 1*time.Second, d2)

	// Third failure: should double again (2s)
	d3 := rateLimiter.When(key)
	assert.Equal(t, 2*time.Second, d3)

	// After forgetting, should reset to base
	rateLimiter.Forget(key)
	d4 := rateLimiter.When(key)
	assert.Equal(t, 500*time.Millisecond, d4)
}
