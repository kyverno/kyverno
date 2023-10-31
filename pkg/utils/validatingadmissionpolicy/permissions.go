package validatingadmissionpolicy

import (
	"context"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// HasRequiredPermissions check if the admission controller has the required permissions to generate both
// validating admission policies and their bindings.
func HasRequiredPermissions(resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(context.TODO(), s, resource.Group, resource.Version, resource.Resource, "", "", "create", "update", "list", "delete")
	if err != nil {
		return false
	}
	return can
}
