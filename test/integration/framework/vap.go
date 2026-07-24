package framework

import (
	"context"

	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GenerateVAP builds a native ValidatingAdmissionPolicy and its binding from a
// Kyverno policy (ClusterPolicy or ValidatingPolicy) using Kyverno's real
// generation code path (pkg/admissionpolicy). It does not apply them to the
// cluster. Any policy that cannot be converted (mutation/generate rules, or a
// validate policy outside the VAP-supported subset) surfaces as an error from
// the underlying builder rather than a partial result.
//
// The VAP is named with the same cpol-/vpol- prefix the admission-policy
// generator controller uses, because BuildValidatingAdmissionPolicyBinding sets
// the binding's PolicyName to that exact value; the names must match or the
// binding references a non-existent policy.
func GenerateVAP(
	discovery dclient.IDiscovery,
	policy engineapi.GenericPolicy,
	exceptions []engineapi.GenericException,
) (*admissionregistrationv1.ValidatingAdmissionPolicy, *admissionregistrationv1.ValidatingAdmissionPolicyBinding, error) {
	name := vapName(policy)
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
	if err := admissionpolicy.BuildValidatingAdmissionPolicy(discovery, vap, policy, exceptions); err != nil {
		return nil, nil, err
	}
	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-binding"},
	}
	if err := admissionpolicy.BuildValidatingAdmissionPolicyBinding(binding, policy); err != nil {
		return nil, nil, err
	}
	return vap, binding, nil
}

// vapName mirrors the admission-policy generator's naming so the VAP name matches
// the PolicyName that BuildValidatingAdmissionPolicyBinding hardcodes.
func vapName(policy engineapi.GenericPolicy) string {
	if policy.AsKyvernoPolicy() != nil {
		return "cpol-" + policy.GetName()
	}
	return "vpol-" + policy.GetName()
}

// ApplyVAP creates the VAP and its binding on the cluster and returns a cleanup
// func that deletes both. If the binding fails to create, the VAP is rolled back
// so no orphan policy is left behind.
//
// The API server activates a ValidatingAdmissionPolicy asynchronously, so callers
// must poll for enforcement (create a violating resource until it is denied)
// rather than assuming the policy is active immediately after this returns.
func ApplyVAP(
	ctx context.Context,
	kube kubernetes.Interface,
	vap *admissionregistrationv1.ValidatingAdmissionPolicy,
	binding *admissionregistrationv1.ValidatingAdmissionPolicyBinding,
) (func(), error) {
	if _, err := kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, vap, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	if _, err := kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		// Roll back with a fresh context: the passed-in ctx may already be the
		// reason the binding create failed (cancelled/timed out), and reusing it
		// would leave the VAP orphaned.
		_ = kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.Background(), vap.Name, metav1.DeleteOptions{})
		return nil, err
	}
	cleanup := func() {
		_ = kube.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(context.Background(), binding.Name, metav1.DeleteOptions{})
		_ = kube.AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(context.Background(), vap.Name, metav1.DeleteOptions{})
	}
	return cleanup, nil
}
