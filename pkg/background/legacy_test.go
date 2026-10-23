package background

import (
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	kyvernoinformers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func newLegacyUR(name string, requestType kyvernov2.RequestType, state kyvernov2.UpdateRequestState) *kyvernov2.UpdateRequest {
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

// TestDropLegacyUpdateRequest_Deletes verifies the UR is deleted (not
// status-written) regardless of its current state.
func TestDropLegacyUpdateRequest_Deletes(t *testing.T) {
	for _, state := range []kyvernov2.UpdateRequestState{"", kyvernov2.Pending, kyvernov2.Failed, kyvernov2.Completed, kyvernov2.Skip} {
		t.Run(string(state), func(t *testing.T) {
			ur := newLegacyUR("legacy-ur", kyvernov2.Generate, state)
			fakeClient := fake.NewSimpleClientset(ur)

			deleteCalled := false
			fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
				deleteCalled = true
				return false, nil, nil
			})

			c := &controller{kyvernoClient: fakeClient}
			err := c.dropLegacyUpdateRequest(ur)

			assert.NoError(t, err)
			assert.True(t, deleteCalled, "dropLegacyUpdateRequest must delete the UR")
		})
	}
}

// TestDropLegacyUpdateRequest_NotFoundIsNil verifies an already-deleted
// legacy UR is treated as success, not an error to retry.
func TestDropLegacyUpdateRequest_NotFoundIsNil(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "updaterequests"}, "legacy-ur")
	})

	c := &controller{kyvernoClient: fakeClient}
	ur := newLegacyUR("legacy-ur", kyvernov2.Mutate, kyvernov2.Pending)

	assert.NoError(t, c.dropLegacyUpdateRequest(ur))
}

// TestDropLegacyUpdateRequest_PropagatesOtherErrors verifies a non-NotFound
// delete failure is returned so handleErr's existing backoff/maxRetries
// machinery retries it, rather than being swallowed.
func TestDropLegacyUpdateRequest_PropagatesOtherErrors(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewServiceUnavailable("unavailable")
	})

	c := &controller{kyvernoClient: fakeClient}
	ur := newLegacyUR("legacy-ur", kyvernov2.Generate, kyvernov2.Pending)

	assert.Error(t, c.dropLegacyUpdateRequest(ur))
}

// TestSyncUpdateRequest_DropsLegacyBeforePendingCheck verifies that legacy
// mutate/generate URs are deleted regardless of their status state, i.e. the
// dispatch happens before the Pending gate in syncUpdateRequest.
func TestSyncUpdateRequest_DropsLegacyBeforePendingCheck(t *testing.T) {
	for _, state := range []kyvernov2.UpdateRequestState{"", kyvernov2.Pending, kyvernov2.Failed, kyvernov2.Completed, kyvernov2.Skip} {
		name := string(state)
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			ur := newLegacyUR("legacy-ur-"+string(state), kyvernov2.Generate, state)
			fakeClient := fake.NewSimpleClientset(ur)
			factory := kyvernoinformers.NewSharedInformerFactory(fakeClient, 0)
			urInformer := factory.Kyverno().V2().UpdateRequests()

			c := &controller{
				kyvernoClient: fakeClient,
				urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
			}

			stop := make(chan struct{})
			defer close(stop)
			factory.Start(stop)
			factory.WaitForCacheSync(stop)

			deleteCalled := false
			fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
				deleteCalled = true
				return false, nil, nil
			})

			err := c.syncUpdateRequest(config.KyvernoNamespace() + "/" + ur.GetName())
			assert.NoError(t, err)
			assert.True(t, deleteCalled, "legacy UR must be dropped regardless of state %q", state)
		})
	}
}

// TestSyncUpdateRequest_CELGenerateNeverDropped is the negative control
// required alongside dropLegacyUpdateRequest: a CEL-generated UR must never
// be routed through the Mutate/Generate-only legacy-delete branch. Status is
// Skip so neither dropLegacyUpdateRequest nor reconcileURStatus's
// delete-on-Completed path fires; a clean run here must produce zero
// deletes.
func TestSyncUpdateRequest_CELGenerateNeverDropped(t *testing.T) {
	ur := newLegacyUR("cel-ur", kyvernov2.CELGenerate, kyvernov2.Skip)
	fakeClient := fake.NewSimpleClientset(ur)
	factory := kyvernoinformers.NewSharedInformerFactory(fakeClient, 0)
	urInformer := factory.Kyverno().V2().UpdateRequests()

	c := &controller{
		kyvernoClient: fakeClient,
		urLister:      urInformer.Lister().UpdateRequests(config.KyvernoNamespace()),
	}

	stop := make(chan struct{})
	defer close(stop)
	factory.Start(stop)
	factory.WaitForCacheSync(stop)

	deleteCalled := false
	fakeClient.PrependReactor("delete", "updaterequests", func(action k8stesting.Action) (bool, runtime.Object, error) {
		deleteCalled = true
		return false, nil, nil
	})

	err := c.syncUpdateRequest(config.KyvernoNamespace() + "/" + ur.GetName())
	assert.NoError(t, err)
	assert.False(t, deleteCalled, "a CEL request type must never be deleted via dropLegacyUpdateRequest")
}
