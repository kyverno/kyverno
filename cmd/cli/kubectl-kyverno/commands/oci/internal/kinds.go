package internal

import (
	"strings"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// AcceptedAPIVersions is the OCI bundle accepted-version table (bundle-spec.md section 3),
// storage version first. v1alpha1 is rejected by omission.
var AcceptedAPIVersions = []string{
	schema.GroupVersion(policiesv1beta1.GroupVersion).String(),
	schema.GroupVersion(policiesv1.GroupVersion).String(),
}

// IsAcceptedAPIVersion reports whether apiVersion is in AcceptedAPIVersions.
func IsAcceptedAPIVersion(apiVersion string) bool {
	for _, v := range AcceptedAPIVersions {
		if v == apiVersion {
			return true
		}
	}
	return false
}

// AcceptedAPIVersionsText renders AcceptedAPIVersions for error messages, e.g.
// "policies.kyverno.io/v1beta1 or policies.kyverno.io/v1".
func AcceptedAPIVersionsText() string {
	return strings.Join(AcceptedAPIVersions, " or ")
}

var LegacyKinds = map[string]bool{
	"Policy":               true,
	"ClusterPolicy":        true,
	"CleanupPolicy":        true,
	"ClusterCleanupPolicy": true,
}

var SupportedCELKinds = map[string]bool{
	"ValidatingPolicy":                true,
	"NamespacedValidatingPolicy":      true,
	"MutatingPolicy":                  true,
	"NamespacedMutatingPolicy":        true,
	"GeneratingPolicy":                true,
	"NamespacedGeneratingPolicy":      true,
	"DeletingPolicy":                  true,
	"NamespacedDeletingPolicy":        true,
	"ImageValidatingPolicy":           true,
	"NamespacedImageValidatingPolicy": true,
	"PolicyException":                 true,
}
