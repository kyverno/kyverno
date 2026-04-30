package policystatus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	auth "github.com/kyverno/kyverno/pkg/auth/checker"
	versionedfake "github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/controllers/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

func TestRetryStatusUpdate_swallowsNotFound(t *testing.T) {
	t.Parallel()
	err := retryStatusUpdate(logr.Discard(), func() error {
		return apierrors.NewNotFound(schema.GroupResource{Resource: "validatingpolicies"}, "missing")
	})
	assert.NoError(t, err)
}

func TestRetryStatusUpdate_conflictsThenSucceeds(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	err := retryStatusUpdate(logr.Discard(), func() error {
		if calls.Add(1) == 1 {
			return apierrors.NewConflict(
				schema.GroupResource{Group: policiesv1beta1.SchemeGroupVersion.Group, Resource: "validatingpolicies"},
				"test",
				errors.New("the object has been modified"),
			)
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "RetryOnConflict should rerun the function after Conflict")
}

func TestReconcile_ValidatingPolicy_retriesOnUpdateStatusConflict(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vpol"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{},
			},
		},
	}

	client := versionedfake.NewSimpleClientset(vpol)

	var statusUpdateCalls atomic.Int32
	client.PrependReactor("update", "validatingpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		if statusUpdateCalls.Add(1) == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: policiesv1beta1.SchemeGroupVersion.Group, Resource: "validatingpolicies"},
				vpol.Name,
				errors.New("the object has been modified"),
			)
		}
		return false, nil, nil
	})

	dc := dclient.NewEmptyFakeClient()
	c := controller{
		dclient:          dc,
		client:           client,
		authChecker:      auth.NewSubjectChecker(dc.GetKubeClient().AuthorizationV1().SubjectAccessReviews(), "", nil),
		polStateRecorder: webhook.NewStateRecorder(nil),
	}

	key := webhook.BuildRecorderKey(webhook.ValidatingPolicyType, vpol.Name, "")
	err := c.reconcile(ctx, logr.Discard(), key, "", "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, statusUpdateCalls.Load(), int32(2), "first UpdateStatus conflict should be retried")

	updated, err := client.PoliciesV1beta1().ValidatingPolicies().Get(ctx, vpol.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, updated.Status.ConditionStatus.Conditions)
}
