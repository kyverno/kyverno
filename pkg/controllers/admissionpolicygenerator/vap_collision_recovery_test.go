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
	"k8s.io/apimachinery/pkg/runtime"
	admissionregistrationv1listers "k8s.io/client-go/listers/admissionregistration/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

func TestReconcileVAPCollisionRecovery(t *testing.T) {
	for _, tc := range []struct {
		name    string
		binding bool
		owners  []metav1.OwnerReference
	}{
		{name: "unowned VAP"},
		{name: "unowned binding", binding: true},
		{name: "foreign VAP", owners: []metav1.OwnerReference{{Kind: "ConfigMap", Name: "foreign", UID: "foreign-uid", APIVersion: "v1"}}},
		{name: "foreign binding", binding: true, owners: []metav1.OwnerReference{{Kind: "ConfigMap", Name: "foreign", UID: "foreign-uid", APIVersion: "v1"}}},
		{name: "missing policy owner", owners: []metav1.OwnerReference{{Kind: "ValidatingPolicy", Name: "missing", UID: "foreign-uid", APIVersion: "policies.kyverno.io/v1beta1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := toggle.NewContext(context.Background(), fakeVAPToggle{Toggles: toggle.FromContext(context.Background())})
			policy := disallowPrivilegeEscalationVpol(nil)
			policy.Spec.AutogenConfiguration.PodControllers.Controllers = []string{}
			nativeName := "vpol-" + policy.Name
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: nativeName, OwnerReferences: tc.owners},
			}
			binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
				ObjectMeta: metav1.ObjectMeta{Name: nativeName + "-binding", OwnerReferences: tc.owners},
			}
			var collision runtime.Object = vap
			if tc.binding {
				collision = binding
			}
			c, kubeClient, kyvernoClient := newVAPTestController(t, policy, collision)
			c.queue = workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[any]())
			t.Cleanup(c.queue.ShutDown)

			key := "ValidatingPolicy/" + policy.Name
			require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", policy.Name))
			stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.False(t, stored.Status.Generated)
			require.Contains(t, stored.Status.GetConditionStatus().Message, "name collision")
			require.Empty(t, kubeClient.Actions(), "collision must not mutate either native resource")
			require.Zero(t, c.queue.Len(), "collision itself must not enqueue a retry")

			// Remove the collision from both the API and informer cache before
			// delivering the delete event, as the informer normally does.
			if tc.binding {
				require.NoError(t, kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, binding.Name, metav1.DeleteOptions{}))
				c.vapbindingLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyBindingLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))
				c.deleteVAPbinding(binding)
			} else {
				require.NoError(t, kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, vap.Name, metav1.DeleteOptions{}))
				c.vapLister = admissionregistrationv1listers.NewValidatingAdmissionPolicyLister(cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{}))
				c.deleteVAP(vap)
			}

			require.Equal(t, 1, c.queue.Len())
			item, shutdown := c.queue.Get()
			require.False(t, shutdown)
			defer c.queue.Done(item)
			require.Equal(t, key, item)
			require.NoError(t, c.reconcile(ctx, logr.Discard(), key, "", policy.Name))

			generatedVAP, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, vap.Name, metav1.GetOptions{})
			require.NoError(t, err)
			generatedBinding, err := kubeClient.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binding.Name, metav1.GetOptions{})
			require.NoError(t, err)
			require.Len(t, generatedVAP.OwnerReferences, 1)
			require.Len(t, generatedBinding.OwnerReferences, 1)
			assert.Equal(t, policy.UID, generatedVAP.OwnerReferences[0].UID)
			assert.Equal(t, policy.UID, generatedBinding.OwnerReferences[0].UID)
			stored, err = kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.True(t, stored.Status.Generated)
			assert.Empty(t, stored.Status.GetConditionStatus().Message)
		})
	}
}

func TestReconcileVAPIdleForeignResources(t *testing.T) {
	for _, tc := range []struct {
		name    string
		vap     bool
		binding bool
		owners  []metav1.OwnerReference
	}{
		{name: "unowned VAP", vap: true},
		{name: "unowned binding", binding: true},
		{name: "both foreign resources", vap: true, binding: true, owners: []metav1.OwnerReference{{Kind: "ConfigMap", Name: "foreign", UID: "foreign-uid", APIVersion: "v1"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := toggle.NewContext(context.Background(), fakeVAPToggle{Toggles: toggle.FromContext(context.Background())})
			policy := disallowPrivilegeEscalationVpol(nil)
			policy.Spec.AutogenConfiguration.ValidatingAdmissionPolicy.Enabled = ptr.To(false)
			policy.Status.GetConditionStatus().Message = "existing status"
			nativeName := "vpol-" + policy.Name
			var objects []runtime.Object
			if tc.vap {
				objects = append(objects, &admissionregistrationv1.ValidatingAdmissionPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: nativeName, OwnerReferences: tc.owners},
				})
			}
			if tc.binding {
				objects = append(objects, &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
					ObjectMeta: metav1.ObjectMeta{Name: nativeName + "-binding", OwnerReferences: tc.owners},
				})
			}
			c, kubeClient, kyvernoClient := newVAPTestController(t, policy, objects...)
			require.NoError(t, c.reconcile(ctx, logr.Discard(), "ValidatingPolicy/"+policy.Name, "", policy.Name))
			assert.Empty(t, kubeClient.Actions(), "idle policy must not manage foreign resources")
			assert.Empty(t, kyvernoClient.Actions(), "idle policy must not write status")
			stored, err := kyvernoClient.PoliciesV1beta1().ValidatingPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, policy.Status, stored.Status)
		})
	}
}
