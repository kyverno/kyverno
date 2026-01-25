package background

import (
    "testing"

    kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
    "github.com/kyverno/kyverno/pkg/config"
    "github.com/stretchr/testify/assert"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/client-go/util/workqueue"
)

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
	"context"
	"errors"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

// TestReconcileURStatus_TransientGetError verifies that when the API Get() call
// fails with a transient error (timeout, connection reset, etc.), reconcileURStatus
// returns the error instead of silently falling back to stale data.
//
// This prevents Completed UpdateRequests from leaking when:
// 1. processUR successfully updates the API status to Completed
// 2. The subsequent Get() in reconcileURStatus fails with a transient error
// 3. Previously, the code would fall back to the stale ur (still showing Pending)
// 4. The switch wouldn't match, errUpdate would be nil, and the UR would leak
func TestReconcileURStatus_TransientGetError(t *testing.T) {
	// Create a fake kyverno client
	kyvernoClient := fake.NewSimpleClientset()

	// Intercept Get calls to UpdateRequests and return a transient error
	transientErr := errors.New("connection reset by peer")
	kyvernoClient.PrependReactor("get", "updaterequests", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, transientErr
	})

	// Create a minimal controller with just the kyvernoClient
	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	// Create a stale UpdateRequest with Pending status
	// This simulates the scenario where processUR has already updated the API
	// status to Completed, but we're still holding the old cached copy
	staleUR := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending, // Stale state - API might already be Completed
		},
	}

	// Call reconcileURStatus with the stale UR
	state, err := c.reconcileURStatus(staleUR)

	// Assert that the function returns an error for transient failures
	assert.Error(t, err, "reconcileURStatus should return an error for transient Get failures")
	assert.Contains(t, err.Error(), "failed to get latest UR status", "error should be wrapped with context")
	assert.Contains(t, err.Error(), transientErr.Error(), "error should contain the original error")

	// The state should be empty since we couldn't fetch the latest status
	assert.Empty(t, state, "state should be empty when Get fails with transient error")
}

// TestReconcileURStatus_ContextDeadlineExceeded verifies handling of context deadline errors
func TestReconcileURStatus_ContextDeadlineExceeded(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	// Simulate context deadline exceeded (a common transient error)
	kyvernoClient.PrependReactor("get", "updaterequests", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, context.DeadlineExceeded
	})

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	staleUR := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-deadline",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	state, err := c.reconcileURStatus(staleUR)

	assert.Error(t, err, "reconcileURStatus should return an error for deadline exceeded")
	assert.ErrorIs(t, err, context.DeadlineExceeded, "error should wrap context.DeadlineExceeded")
	assert.Empty(t, state)
}

// TestReconcileURStatus_NotFoundError verifies that NotFound errors are handled gracefully
// (the UR was deleted externally, so there's nothing to reconcile)
func TestReconcileURStatus_NotFoundError(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()
	// Don't add any UpdateRequest to the client - Get will return NotFound

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	staleUR := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deleted-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Pending,
		},
	}

	state, err := c.reconcileURStatus(staleUR)

	// NotFound should NOT return an error - the UR is already gone
	assert.NoError(t, err, "reconcileURStatus should not return an error for NotFound")
	assert.Empty(t, state, "state should be empty when UR is not found")
}

// TestReconcileURStatus_CompletedState verifies that Completed URs are deleted
func TestReconcileURStatus_CompletedState(t *testing.T) {
	// Create an UpdateRequest with Completed status
	completedUR := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "completed-ur",
			Namespace: "kyverno",
		},
		Status: kyvernov2.UpdateRequestStatus{
			State: kyvernov2.Completed,
		},
	}

	kyvernoClient := fake.NewSimpleClientset(completedUR)

	// Track if delete was called
	deleteCalled := false
	kyvernoClient.PrependReactor("delete", "updaterequests", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteCalled = true
		return true, nil, nil
	})

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	state, err := c.reconcileURStatus(completedUR)

	assert.NoError(t, err)
	assert.Equal(t, kyvernov2.Completed, state)
	assert.True(t, deleteCalled, "Completed UR should be deleted")
}
