package admissionpolicy

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
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicies"}
	return hasPermissions(gvr, s)
}

// HasValidatingAdmissionPolicyBindingPermission check if the admission controller has the required permissions to generate
// Kubernetes ValidatingAdmissionPolicyBinding
func HasValidatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1", Resource: "validatingadmissionpolicybindings"}
	return hasPermissions(gvr, s)
}

// HasMutatingAdmissionPolicyPermission check if the admission controller has the required permissions to generate
// Kubernetes MutatingAdmissionPolicy
func HasMutatingAdmissionPolicyPermission(s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1alpha1", Resource: "mutatingadmissionpolicies"}
	return hasPermissions(gvr, s)
}

// HasMutatingAdmissionPolicyBindingPermission check if the admission controller has the required permissions to generate
// Kubernetes MutatingAdmissionPolicyBinding
func HasMutatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: "v1alpha1", Resource: "mutatingadmissionpolicybindings"}
	return hasPermissions(gvr, s)
}

// IsMutatingAdmissionPolicyRegistered checks if MutatingAdmissionPolicies are registered in the API Server
func IsMutatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	groupVersion := schema.GroupVersion{Group: "admissionregistration.k8s.io", Version: "v1alpha1"}
	if _, err := kubeClient.Discovery().ServerResourcesForGroupVersion(groupVersion.String()); err != nil {
		return false, err
	}
	return true, nil
}
