package background

import (
	"context"
	"testing"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestEmptyStateRecoveryWait(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		created time.Time
		want    time.Duration
	}{
		{name: "unset timestamp", created: time.Time{}, want: 0},
		{name: "just created", created: now, want: emptyStateRecoveryDelay},
		{name: "clock skew", created: now.Add(time.Minute), want: emptyStateRecoveryDelay},
		{name: "inside the window", created: now.Add(-2 * time.Second), want: emptyStateRecoveryDelay - 2*time.Second},
		{name: "window elapsed", created: now.Add(-emptyStateRecoveryDelay), want: 0},
		{name: "older than the window", created: now.Add(-emptyStateRecoveryDelay - time.Second), want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, emptyStateRecoveryWait(tt.created, now))
		})
	}
}

func TestReconcileURStatus_RecoversOldEmptyState(t *testing.T) {
	for _, requestType := range []kyvernov2.RequestType{kyvernov2.CELGenerate, kyvernov2.CELMutate} {
		t.Run(string(requestType), func(t *testing.T) {
			ur := newTestUpdateRequest("empty-ur", "", requestType)
			ur.CreationTimestamp = metav1.NewTime(time.Now().Add(-emptyStateRecoveryDelay - time.Second))
			client := fake.NewSimpleClientset(ur)

			updateCalled := false
			client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
				assert.Equal(t, "status", action.GetSubresource())
				updated := action.(k8stesting.UpdateAction).GetObject().(*kyvernov2.UpdateRequest)
				assert.Equal(t, kyvernov2.Pending, updated.Status.State)
				assert.Equal(t, ur.Spec, updated.Spec)
				updateCalled = true
				return false, nil, nil
			})

			c := &controller{kyvernoClient: client}
			state, err := c.reconcileURStatus(ur)
			assert.NoError(t, err)
			assert.Equal(t, kyvernov2.Pending, state)
			assert.True(t, updateCalled, "an old empty request must be set to Pending")

			stored, getErr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.Background(), ur.GetName(), metav1.GetOptions{})
			assert.NoError(t, getErr)
			assert.Equal(t, kyvernov2.Pending, stored.Status.State)
			assert.Equal(t, ur.Spec, stored.Spec)
		})
	}
}

func TestReconcileURStatus_EmptyRecoveryErrorIsReturned(t *testing.T) {
	ur := newTestUpdateRequest("empty-ur", "", kyvernov2.CELGenerate)
	ur.CreationTimestamp = metav1.NewTime(time.Now().Add(-emptyStateRecoveryDelay - time.Second))
	client := fake.NewSimpleClientset(ur)
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		assert.Equal(t, "status", action.GetSubresource())
		return true, nil, apierrors.NewServiceUnavailable("unavailable")
	})

	c := &controller{kyvernoClient: client}
	state, err := c.reconcileURStatus(ur)
	assert.Error(t, err)
	assert.Equal(t, kyvernov2.Pending, state)

	stored, getErr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.Background(), ur.GetName(), metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Empty(t, stored.Status.State, "a failed status write must leave the stored request empty so the retry can recover it")
}

func TestReconcileURStatus_LeavesYoungEmptyState(t *testing.T) {
	now := time.Now()
	ur := newTestUpdateRequest("young-ur", "", kyvernov2.CELGenerate)
	ur.CreationTimestamp = metav1.NewTime(now.Add(-emptyStateRecoveryDelay + 25*time.Millisecond))
	client := fake.NewSimpleClientset(ur)

	updateCalled := false
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateCalled = true
		return false, nil, nil
	})

	c := &controller{
		kyvernoClient: client,
		queue:         newTestQueue(),
	}
	t.Cleanup(c.queue.ShutDown)

	state, err := c.reconcileURStatusAt(ur, now)
	assert.NoError(t, err)
	assert.Empty(t, state)
	assert.False(t, updateCalled, "a young empty request is still inside the creator's Pending write")
	assert.Equal(t, 0, c.queue.Len())
	assert.Eventually(t, func() bool { return c.queue.Len() == 1 }, time.Second, 5*time.Millisecond)

	stored, getErr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.Background(), ur.GetName(), metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Empty(t, stored.Status.State)
}

func TestReconcileURStatus_EmptyCacheDoesNotOverwriteLiveState(t *testing.T) {
	for _, live := range []kyvernov2.UpdateRequestState{kyvernov2.Pending, kyvernov2.Skip} {
		t.Run(string(live), func(t *testing.T) {
			ur := newTestUpdateRequest("live-ur", live, kyvernov2.CELGenerate)
			client := fake.NewSimpleClientset(ur)

			updateCalled := false
			client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
				updateCalled = true
				return false, nil, nil
			})

			stale := ur.DeepCopy()
			stale.Status.State = ""
			c := &controller{kyvernoClient: client}
			state, err := c.reconcileURStatus(stale)
			assert.NoError(t, err)
			assert.Equal(t, live, state)
			assert.False(t, updateCalled, "recovery must follow the live object, not the empty informer copy")
		})
	}
}

func TestReconcileURStatus_EmptyCacheStillDeletesLiveCompleted(t *testing.T) {
	ur := newTestUpdateRequest("done-ur", kyvernov2.Completed, kyvernov2.CELGenerate)
	client := fake.NewSimpleClientset(ur)

	deleteCalled := false
	updateCalled := false
	client.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteCalled = true
		return true, nil, nil
	})
	client.PrependReactor("update", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		updateCalled = true
		return false, nil, nil
	})

	stale := ur.DeepCopy()
	stale.Status.State = ""
	c := &controller{kyvernoClient: client}
	state, err := c.reconcileURStatus(stale)
	assert.NoError(t, err)
	assert.Equal(t, kyvernov2.Completed, state)
	assert.True(t, deleteCalled)
	assert.False(t, updateCalled)
}

func TestSyncUpdateRequest_RecoversOldEmptyCELRequest(t *testing.T) {
	ur := newTestUpdateRequest("cel-empty", "", kyvernov2.CELGenerate)
	ur.CreationTimestamp = metav1.NewTime(time.Now().Add(-emptyStateRecoveryDelay - time.Second))
	c, client := celControllerWithUR(t, ur)

	err := c.syncUpdateRequest(config.KyvernoNamespace() + "/" + ur.GetName())
	assert.NoError(t, err)

	stored, getErr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.Background(), ur.GetName(), metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Equal(t, kyvernov2.Pending, stored.Status.State)
	assert.Equal(t, ur.Spec, stored.Spec)
}

func TestSyncUpdateRequest_LeavesYoungEmptyCELRequest(t *testing.T) {
	ur := newTestUpdateRequest("cel-young", "", kyvernov2.CELMutate)
	// A future timestamp stays inside the wait even if the informer sync is slow.
	ur.CreationTimestamp = metav1.NewTime(time.Now().Add(time.Minute))
	c, client := celControllerWithUR(t, ur)

	err := c.syncUpdateRequest(config.KyvernoNamespace() + "/" + ur.GetName())
	assert.NoError(t, err)

	stored, getErr := client.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.Background(), ur.GetName(), metav1.GetOptions{})
	assert.NoError(t, getErr)
	assert.Empty(t, stored.Status.State)
	assert.Equal(t, ur.Spec, stored.Spec)
}

func TestRequeueUpdateRequest_NilQueue(t *testing.T) {
	c := &controller{}
	c.requeueUpdateRequest(newTestUpdateRequest("ur", "", kyvernov2.CELGenerate), time.Second)
}

func celControllerWithUR(t *testing.T, ur *kyvernov2.UpdateRequest) (*controller, *fake.Clientset) {
	t.Helper()
	client := fake.NewSimpleClientset(ur)
	factory := kyvernoinformers.NewSharedInformerFactory(client, 0)
	urInformer := factory.Kyverno().V2().UpdateRequests()
	c := &controller{
		kyvernoClient: client,
		urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
		queue:         newTestQueue(),
	}
	t.Cleanup(c.queue.ShutDown)

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })
	factory.Start(stop)
	for typ, ok := range factory.WaitForCacheSync(stop) {
		if !ok {
			t.Fatalf("informer cache did not sync: %v", typ)
		}
	}
	return c, client
}
