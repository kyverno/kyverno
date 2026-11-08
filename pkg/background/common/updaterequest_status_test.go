package common

import (
	"context"
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
)

func newTestUR(name string, state kyvernov2.UpdateRequestState, retryCount int) *kyvernov2.UpdateRequest {
	return &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: config.KyvernoNamespace(),
		},
		Status: kyvernov2.UpdateRequestStatus{
			State:      state,
			RetryCount: retryCount,
		},
	}
}

func statusUpdates(actions []k8stesting.Action) int {
	count := 0
	for _, a := range actions {
		if a.GetVerb() == "update" && a.GetSubresource() == "status" {
			count++
		}
	}
	return count
}

func deletes(actions []k8stesting.Action) int {
	count := 0
	for _, a := range actions {
		if a.GetVerb() == "delete" {
			count++
		}
	}
	return count
}

// Regression test for https://github.com/kyverno/kyverno/issues/18499.
// Once the retry budget is spent, retryOrDeleteOnFailure deletes the request.
// UpdateStatus must not then write the status subresource of the deleted
// object: the write 404s and the terminal cleanup is reported as a
// reconciliation error, which spams logs, error metrics and requeues.
func TestUpdateStatus_RetryExhaustedDeletesWithoutStatusWrite(t *testing.T) {
	ur := newTestUR("ur-exhausted", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)

	got, err := UpdateStatus(context.TODO(), client, nil, "ur-exhausted", kyvernov2.Failed, "boom", nil)
	assert.NoError(t, err, "terminal delete must not surface the 404 from a status write on the deleted object")
	assert.NotNil(t, got, "callers still expect the pre-update object back")

	// the request is gone
	_, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-exhausted", metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(gerr), "exhausted request should be deleted, got %v", gerr)

	actions := client.Actions()
	assert.Equal(t, 1, deletes(actions), "expected exactly one delete action")
	assert.Equal(t, 0, statusUpdates(actions), "no status write may happen after the delete")
}

// One retry below the limit keeps the request and bumps RetryCount.
func TestUpdateStatus_BelowRetryLimitIncrementsCount(t *testing.T) {
	ur := newTestUR("ur-retry", kyvernov2.Pending, 1)
	client := fake.NewSimpleClientset(ur)

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-retry", kyvernov2.Failed, "boom", nil)
	assert.NoError(t, err)

	updated, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-retry", metav1.GetOptions{})
	assert.NoError(t, gerr)
	assert.Equal(t, kyvernov2.Failed, updated.Status.State)
	assert.Equal(t, "boom", updated.Status.Message)
	assert.Equal(t, 2, updated.Status.RetryCount, "retry count should increment when below the limit")

	actions := client.Actions()
	assert.Equal(t, 0, deletes(actions), "no delete below the retry limit")
	assert.Equal(t, 1, statusUpdates(actions), "expected one status write for the retry")
}

// Sitting exactly at the limit is still a retry; only exceeding it deletes.
func TestUpdateStatus_AtRetryLimitStillRetries(t *testing.T) {
	ur := newTestUR("ur-at-limit", kyvernov2.Pending, 3)
	client := fake.NewSimpleClientset(ur)

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-at-limit", kyvernov2.Failed, "boom", nil)
	assert.NoError(t, err)

	updated, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-at-limit", metav1.GetOptions{})
	assert.NoError(t, gerr)
	assert.Equal(t, 4, updated.Status.RetryCount)

	actions := client.Actions()
	assert.Equal(t, 0, deletes(actions), "delete happens only once the limit is exceeded")
}

// A failing delete must propagate and must not be followed by a status write.
func TestUpdateStatus_PropagatesDeleteFailure(t *testing.T) {
	ur := newTestUR("ur-delete-fails", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("etcdserver: request timed out")
	})

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-delete-fails", kyvernov2.Failed, "boom", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds retry limit")

	assert.Equal(t, 0, statusUpdates(client.Actions()), "no status write when the delete itself failed")

	// the request is still there, the controller will retry the cleanup
	_, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-delete-fails", metav1.GetOptions{})
	assert.NoError(t, gerr)
}

// Non-failure states never consult the retry budget and never delete.
func TestUpdateStatus_SuccessWritesStatusOnly(t *testing.T) {
	ur := newTestUR("ur-success", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-success", kyvernov2.Completed, "", nil)
	assert.NoError(t, err)

	updated, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur-success", metav1.GetOptions{})
	assert.NoError(t, gerr)
	assert.Equal(t, kyvernov2.Completed, updated.Status.State)
	assert.Equal(t, 4, updated.Status.RetryCount, "success leaves the retry counter untouched")

	actions := client.Actions()
	assert.Equal(t, 0, deletes(actions), "success never deletes")
	assert.Equal(t, 1, statusUpdates(actions))
}

// The Get of a vanished request keeps its dedicated error message so callers
// can distinguish it from write failures.
func TestUpdateStatus_GetNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-gone", kyvernov2.Failed, "boom", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch update request")
	assert.Equal(t, 0, statusUpdates(client.Actions()))
}

func TestRetryOrDeleteOnFailure(t *testing.T) {
	tests := []struct {
		name        string
		retryCount  int
		limit       int
		wantDeleted bool
		wantCount   int
	}{{
		name:        "below limit retries",
		retryCount:  0,
		limit:       3,
		wantDeleted: false,
		wantCount:   1,
	}, {
		name:        "at limit retries",
		retryCount:  3,
		limit:       3,
		wantDeleted: false,
		wantCount:   4,
	}, {
		name:        "over limit deletes",
		retryCount:  4,
		limit:       3,
		wantDeleted: true,
		wantCount:   4,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ur := newTestUR("ur", kyvernov2.Pending, tt.retryCount)
			client := fake.NewSimpleClientset(ur)

			latest, deleted, err := retryOrDeleteOnFailure(context.TODO(), client, ur, tt.limit)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantDeleted, deleted)
			assert.NotNil(t, latest)
			assert.Equal(t, tt.wantCount, latest.Status.RetryCount)

			if tt.wantDeleted {
				_, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), "ur", metav1.GetOptions{})
				assert.True(t, apierrors.IsNotFound(gerr))
			}
		})
	}
}

// A delete error is wrapped with the retry context and flags no deletion.
func TestRetryOrDeleteOnFailure_DeleteError(t *testing.T) {
	ur := newTestUR("ur", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"}, "ur", errors.New("conflict"))
	})

	latest, deleted, err := retryOrDeleteOnFailure(context.TODO(), client, ur, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds retry limit")
	assert.False(t, deleted)
	assert.Nil(t, latest)
}

// A NotFound from the terminal delete means another worker or manual cleanup
// already removed the request between the fetch and the delete. The terminal
// state is reached either way, so the race reports deleted with no error
// instead of surfacing a requeue-worthy failure.
func TestRetryOrDeleteOnFailure_DeleteNotFoundCountsAsDeleted(t *testing.T) {
	ur := newTestUR("ur", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		// Model the competing cleanup: the request is really gone, and the
		// API reports NotFound to the late delete.
		assert.NoError(t, client.Tracker().Delete(action.GetResource(), action.GetNamespace(), ur.Name))
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"}, "ur")
	})

	latest, deleted, err := retryOrDeleteOnFailure(context.TODO(), client, ur, 3)
	assert.NoError(t, err, "an already-deleted request is the desired end state, not a failure")
	assert.True(t, deleted, "NotFound still means the request is gone")
	assert.NotNil(t, latest, "callers still expect the pre-delete object back")
	_, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), ur.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(gerr), "the competing cleanup must actually remove the request")
}

// UpdateStatus must not report failure nor attempt a status write when the
// terminal delete loses the race to another cleanup.
func TestUpdateStatus_DeleteNotFoundSkipsStatusWrite(t *testing.T) {
	ur := newTestUR("ur-gone-mid-delete", kyvernov2.Pending, 4)
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		assert.NoError(t, client.Tracker().Delete(action.GetResource(), action.GetNamespace(), ur.Name))
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"}, "ur-gone-mid-delete")
	})

	_, err := UpdateStatus(context.TODO(), client, nil, "ur-gone-mid-delete", kyvernov2.Failed, "boom", nil)
	assert.NoError(t, err, "losing the delete race must not requeue or log a failure")
	assert.Equal(t, 0, statusUpdates(client.Actions()), "no status write when the request is already gone")
	_, gerr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), ur.Name, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(gerr), "the competing cleanup must actually remove the request")
}
