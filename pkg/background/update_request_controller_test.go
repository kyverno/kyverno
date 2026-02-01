package background

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
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
}
