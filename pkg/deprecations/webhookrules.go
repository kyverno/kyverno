package deprecations

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

// LegacyPolicyRule matches create and update requests for the legacy kyverno.io ClusterPolicy
// and Policy kinds. Keep APIVersions as "v1" and "v2beta1" unchanged in 1.20 so admission
// coverage of legacy writes is not altered; the engine returns the 1.20 hard error on those
// writes (see BuildKindError and #17491). The legacy versions are removed in 1.21.
var LegacyPolicyRule = admissionregistrationv1.Rule{
	Resources:   []string{"clusterpolicies", "policies"},
	APIGroups:   []string{"kyverno.io"},
	APIVersions: []string{"v1", "v2beta1"},
}

// LegacyPolicyStatusRule matches only update requests to the /status subresource of the legacy
// ClusterPolicy and Policy kinds, so Kyverno's own status writers (transitionally allowed, see
// AllowSubresourceForLegacyStatusWriter) reach the same admission gate as any other write.
var LegacyPolicyStatusRule = admissionregistrationv1.Rule{
	Resources:   []string{"clusterpolicies/status", "policies/status"},
	APIGroups:   []string{"kyverno.io"},
	APIVersions: []string{"v1", "v2beta1"},
}

// LegacyExceptionRule matches create and update requests for the legacy kyverno.io
// PolicyException kind.
var LegacyExceptionRule = admissionregistrationv1.Rule{
	APIGroups: []string{"kyverno.io"},
	// v2 is the storage version for policyexceptions.kyverno.io; v2alpha1 is kept here for
	// backwards compatibility even though no v2alpha1 PolicyException type has ever existed.
	// Without v2, a "kyverno.io/v2" PolicyException (the version most manifests actually use)
	// never reaches this webhook at all.
	APIVersions: []string{"v2", "v2alpha1", "v2beta1"},
	Resources:   []string{"policyexceptions"},
}

// LegacyCleanupPolicyRule matches create and update requests for the legacy kyverno.io
// CleanupPolicy and ClusterCleanupPolicy kinds, including their /status subresource. The "/*"
// suffix is required, not merely permissive: per the apiserver's rule matcher
// (k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules), "<resource>/*" matches both
// the base resource (subresource "") and every subresource, so this single entry covers
// top-level writes and "/status" alike.
var LegacyCleanupPolicyRule = admissionregistrationv1.Rule{
	APIGroups: []string{"kyverno.io"},
	// v2 is the storage version for cleanuppolicies/clustercleanuppolicies.kyverno.io. Without
	// it, a "kyverno.io/v2" CleanupPolicy (the version most manifests actually use) never
	// reaches this webhook at all.
	APIVersions: []string{"v2", "v2beta1"},
	Resources: []string{
		"cleanuppolicies/*",
		"clustercleanuppolicies/*",
	},
}
