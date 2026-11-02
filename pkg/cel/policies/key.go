package policies

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicyKey returns the stable identifier of a policy: namespace/name for
// namespaced policies and the bare name for clustered policies.
func PolicyKey(policy metav1.Object) string {
	if policy.GetNamespace() == "" {
		return policy.GetName()
	}
	return policy.GetNamespace() + "/" + policy.GetName()
}
