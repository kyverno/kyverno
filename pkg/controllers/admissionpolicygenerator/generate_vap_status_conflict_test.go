package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

// TestReconcileVAPDisabledCleanupStatusConflictRecovery proves that cleanup
// propagates an UpdateStatus conflict even after deleting native resources.
// A subsequent reconciliation must recover with stale informer snapshots.
func TestReconcileVAPDisabledCleanupStatusConflictRecovery(t *testing.T) {
	baseCtx := context.Background()
	ctx := toggle.NewContext(baseCtx, fakeVAPToggle{
		Toggles: toggle.FromContext(baseCtx),
		generateValidatingAdmissionPolicy: false,
	})
	policy := disallowPrivilegeEscalationVpol(nil)
	policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
	policy.Status.Generated = true

	vapName := "vpol-" + policy.Name
	bindingName := vapName + "-binding"
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: vapName},
	}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
	}
	c, kubeClient, kyvernoClient := newVAPTestController(t, policy, vap, binding)

	statusWrites := 0
	kyvernoClient.PrependReactor("update", "validatingpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		statusWrites++
		if statusWrites == 1 {
			return true, nil, apierrors.NewConflict(
				schema.GroupResource{Group: "policies.kyverno.io", Resource: "validatingpolicies"},
				policy.Name,
				errors.New("simulated status conflict"),
			)
		}
		return false, nil, nil
	})

	key := "ValidatingPolicy/" + policy.Name
	err := c.reconcile(ctx, logr.Discard(), key, "", policy.Name)
	require.Error(t, err)
	assert.True(t, apierrors.IsConflict(err), "status conflict should propagate: %v", err)

	_, vapErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vapName, metav1.GetOptions{})
	_, bindingErr := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(vapErr), "VAP should already be deleted: %v", vapErr)
	assert.True(t, apierrors.IsNotFound(bindingErr), "binding should already be deleted: %v", bindingErr)

	// The listers intentionally remain stale. Repeated delete NotFound must not
	// block recovery of the previously failed status update.
	require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", policy.Name))
	assert.Equal(t, 2, statusWrites, "status update must be retried")
	stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.False(t, stored.Status.Generated)
}
