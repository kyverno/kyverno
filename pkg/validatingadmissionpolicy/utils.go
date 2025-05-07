package validatingadmissionpolicy

import (
	"context"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

func hasPermissions(resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(context.TODO(), s, resource.Group, resource.Version, resource.Resource, "", "", "create", "update", "list", "delete")
	if err != nil {
		return false
	}
	return can
}

// HasValidatingAdmissionPolicyPermission check if the admission controller has the required permissions to generate
// Kubernetes ValidatingAdmissionPolicy
func HasValidatingAdmissionPolicyPermission(s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1beta1", Resource: "validatingadmissionpolicies"}
	return hasPermissions(gvr, s)
}

// HasValidatingAdmissionPolicyBindingPermission check if the admission controller has the required permissions to generate
// Kubernetes ValidatingAdmissionPolicyBinding
func HasValidatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1beta1", Resource: "validatingadmissionpolicybindings"}
	return hasPermissions(gvr, s)
}

// IsValidatingAdmissionPolicyRegistered checks if ValidatingAdmissionPolicies are registered in the API Server
func IsValidatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	groupVersion := schema.GroupVersion{Group: "admissionregistration.k8s.io", Version: "v1beta1"}
	if _, err := kubeClient.Discovery().ServerResourcesForGroupVersion(groupVersion.String()); err != nil {
		return false, err
	}
	return true, nil
}
