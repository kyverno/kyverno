package background

import (
	"errors"
	"testing"

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

	completedUR := newTestUpdateRequest("completed-ur", kyvernov2.Completed, kyvernov2.Generate)

	// updateUR should not enqueue completed URs
	c.updateUR(nil, completedUR)

	// Queue should be empty since completed URs are skipped
	assert.Equal(t, 0, c.queue.Len())
}

func TestUpdateUR_SkipsSkipState(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	skipUR := newTestUpdateRequest("skip-ur", kyvernov2.Skip, kyvernov2.Generate)

	// updateUR should not enqueue skip URs
	c.updateUR(nil, skipUR)

	// Queue should be empty since skip URs are skipped
	assert.Equal(t, 0, c.queue.Len())
}

func TestUpdateUR_EnqueuesPendingState(t *testing.T) {
	c := &controller{
		queue: newTestQueue(),
	}

	pendingUR := newTestUpdateRequest("pending-ur", kyvernov2.Pending, kyvernov2.Generate)

	// updateUR should enqueue pending URs
	c.updateUR(nil, pendingUR)

	// Queue should have the UR
	assert.Equal(t, 1, c.queue.Len())
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
