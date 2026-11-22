package admissionpolicygenerator

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// TestReconcileVAPForeignOwnerCollision ensures a ValidatingPolicy cannot use
// the predictable vpol-<name> convention to delete or overwrite a native
// admission policy it does not own. Both objects must be checked before any
// destructive operation so a partial collision cannot delete the other one.
func TestReconcileVAPForeignOwnerCollision(t *testing.T) {
	for _, tc := range []struct {
		name       string
		foreignVAP bool
		noVAPOwner bool
		foreignBinding bool
		noBindingOwner bool
		enabled bool
	}{
		{name:"foreign policy owner",foreignVAP:true},
		{name:"unowned native policy",noVAPOwner:true},
		{name:"foreign binding owner",foreignBinding:true},
		{name:"unowned binding",noBindingOwner:true},
		{name:"foreign native policy while generation enabled",foreignVAP:true,enabled:true},
		{name:"foreign binding while generation enabled",foreignBinding:true,enabled:true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseCtx := context.Background()
			ctx := toggle.NewContext(baseCtx, fakeVAPToggle{
				Toggles:                           toggle.FromContext(baseCtx),
				generateValidatingAdmissionPolicy: false,
			})
			policy := disallowPrivilegeEscalationVpol(nil)
			policy.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(tc.enabled)
			policy.Status.Generated = true

			vapOwners := vpolOwnerReferences(policy)
			bindingOwners := vpolOwnerReferences(policy)
			if tc.noVAPOwner {
				vapOwners = nil
			}
			if tc.foreignVAP {
				vapOwners[0].UID = "some-other-policy-uid"
			}
			if tc.noBindingOwner {
				bindingOwners = nil
			}
			if tc.foreignBinding {
				bindingOwners[0].UID = "some-other-policy-uid"
			}
			nativeName := "vpol-" + policy.Name
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: nativeName, OwnerReferences: vapOwners},
			}
			binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: nativeName + "-binding", OwnerReferences: bindingOwners},
			}
			c, kubeClient, kyvernoClient := newVAPTestController(t, policy, vap, binding)

			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+policy.Name, "", policy.Name))
			storedVAP, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, nativeName, metav1.GetOptions{})
			require.NoError(t, err, "foreign or unowned native policy must be preserved")
			storedBinding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, nativeName+"-binding", metav1.GetOptions{})
			require.NoError(t, err, "binding must not be deleted during collision")
			assert.Equal(t, vapOwners, storedVAP.OwnerReferences)
			assert.Equal(t, bindingOwners, storedBinding.OwnerReferences)
			for _, action := range kubeClient.Actions() {
				assert.NotEqual(t, "delete", action.GetVerb())
				assert.NotEqual(t, "update", action.GetVerb())
			}
			updated, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.False(t, updated.Status.Generated)
			assert.Contains(t, updated.Status.GetConditionStatus().Message, "name collision")
		})
	}
}
