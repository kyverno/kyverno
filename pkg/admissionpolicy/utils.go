package admissionpolicy

import (
	"context"
	"fmt"
	"slices"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

type MutatingAdmissionPolicyVersion string

const (
	MutatingAdmissionPolicyVersionV1       MutatingAdmissionPolicyVersion = "v1"
	MutatingAdmissionPolicyVersionV1alpha1 MutatingAdmissionPolicyVersion = "v1alpha1"
	MutatingAdmissionPolicyVersionV1beta1  MutatingAdmissionPolicyVersion = "v1beta1"
)

var mutatingAdmissionPolicyVersions = []MutatingAdmissionPolicyVersion{
	MutatingAdmissionPolicyVersionV1,
	MutatingAdmissionPolicyVersionV1beta1,
	MutatingAdmissionPolicyVersionV1alpha1,
}

var errMutatingAdmissionPolicyNotRegistered = fmt.Errorf("mutating admission policy API is not registered")

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

// HasMutatingAdmissionPolicyPermission checks whether the admission controller can manage
// MutatingAdmissionPolicies in any supported API version.
func HasMutatingAdmissionPolicyPermission(s checker.AuthChecker) bool {
	for _, version := range mutatingAdmissionPolicyVersions {
		if HasMutatingAdmissionPolicyPermissionForVersion(version, s) {
			return true
		}
	}
	return false
}

// HasMutatingAdmissionPolicyBindingPermission checks whether the admission controller can manage
// MutatingAdmissionPolicyBindings in any supported API version.
func HasMutatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	for _, version := range mutatingAdmissionPolicyVersions {
		if HasMutatingAdmissionPolicyBindingPermissionForVersion(version, s) {
			return true
		}
	}
	return false
}

func HasMutatingAdmissionPolicyPermissionForVersion(version MutatingAdmissionPolicyVersion, s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: string(version), Resource: "mutatingadmissionpolicies"}
	return hasPermissions(gvr, s)
}

func HasMutatingAdmissionPolicyBindingPermissionForVersion(version MutatingAdmissionPolicyVersion, s checker.AuthChecker) bool {
	gvr := schema.GroupVersionResource{Group: "admissionregistration.k8s.io", Version: string(version), Resource: "mutatingadmissionpolicybindings"}
	return hasPermissions(gvr, s)
}

func isRegistered(kubeClient kubernetes.Interface, group, version string, resources ...string) (bool, error) {
	resourceList, err := kubeClient.Discovery().ServerResourcesForGroupVersion(schema.GroupVersion{Group: group, Version: version}.String())
	if err != nil {
		return false, err
	}
	available := make([]string, 0, len(resourceList.APIResources))
	for _, resource := range resourceList.APIResources {
		available = append(available, resource.Name)
	}
	for _, resource := range resources {
		if !slices.Contains(available, resource) {
			return false, nil
		}
	}
	return true, nil
}

// PreferredMutatingAdmissionPolicyVersion compares the kyverno-supported list of MAP versions to the cluster's versions
// and returns the latest available one
func PreferredMutatingAdmissionPolicyVersion(kubeClient kubernetes.Interface) (MutatingAdmissionPolicyVersion, error) {
	for _, version := range mutatingAdmissionPolicyVersions {
		registered, err := isRegistered(
			kubeClient,
			"admissionregistration.k8s.io",
			string(version),
			"mutatingadmissionpolicies",
			"mutatingadmissionpolicybindings",
		)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return "", err
			}
			continue
		}
		if registered {
			return version, nil
		}
	}
	return "", errMutatingAdmissionPolicyNotRegistered
}

// IsMutatingAdmissionPolicyRegistered checks if MutatingAdmissionPolicies are registered in the API Server.
// It checks supported versions in preference order: v1, then v1beta1, then v1alpha1.
func IsMutatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	_, err := PreferredMutatingAdmissionPolicyVersion(kubeClient)
	if err != nil {
		return false, err
	}
	return true, nil
}

// IsValidatingAdmissionPolicyRegistered checks if ValidatingAdmissionPolicies are registered in the API Server.
// It checks for v1 only since callers wire v1 informers.
func IsValidatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	registered, err := isRegistered(kubeClient, "admissionregistration.k8s.io", "v1", "validatingadmissionpolicies", "validatingadmissionpolicybindings")
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if registered {
		return true, nil
	}
	return false, nil
}

// CollectParams retains the native Kubernetes admission-policy parameter semantics.
func CollectParams(ctx context.Context, client engineapi.Client, paramKind *admissionregistrationv1.ParamKind, paramRef *admissionregistrationv1.ParamRef, namespace string) ([]runtime.Object, error) {
	return CollectParamsForPolicy(ctx, client, paramKind, paramRef, namespace, &kyvernov1.ClusterPolicy{})
}

// CollectParamsForPolicy collects parameter resources while confining a
// namespaced Policy to parameter resources in its own namespace. Every caller
// must provide an explicit policy scope, including cluster-scoped policies.
func CollectParamsForPolicy(
	ctx context.Context,
	client engineapi.Client,
	paramKind *admissionregistrationv1.ParamKind,
	paramRef *admissionregistrationv1.ParamRef,
	resourceNamespace string,
	policy engineapi.PolicyScope,
) ([]runtime.Object, error) {
	policyNamespace, err := engineapi.PolicyNamespace(policy)
	if err != nil {
		return nil, err
	}

	var params []runtime.Object

	apiVersion := paramKind.APIVersion
	kind := paramKind.Kind
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse the parameter resource group version")
	}

	// A namespaced Policy is confined to namespaced parameters in the policy
	// namespace. Other callers retain Kubernetes ValidatingAdmissionPolicy
	// semantics: cluster-scoped params have no namespace and namespaced params
	// default to the admitted resource's namespace.
	var paramsNamespace string
	isNamespaced, err := client.IsNamespaced(gv.Group, gv.Version, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to check if resource is namespaced or not (%w)", err)
	}

	// check if `paramKind` is namespace-scoped
	if isNamespaced {
		if policyNamespace != "" {
			if paramRef.Namespace != "" && paramRef.Namespace != policyNamespace {
				return nil, fmt.Errorf("paramRef.namespace %q must match policy namespace %q", paramRef.Namespace, policyNamespace)
			}
			paramsNamespace = policyNamespace
		} else if paramRef.Namespace != "" {
			paramsNamespace = paramRef.Namespace
		} else if resourceNamespace != "" {
			paramsNamespace = resourceNamespace
		} else {
			return nil, fmt.Errorf("can't use namespaced paramRef to match cluster-scoped resources")
		}
	} else {
		if policyNamespace != "" {
			return nil, fmt.Errorf("cluster-scoped paramKind is not allowed in namespaced policies")
		}
		// It isn't allowed to set namespace for cluster-scoped params
		if paramRef.Namespace != "" {
			return nil, fmt.Errorf("paramRef.namespace must not be provided for a cluster-scoped `paramKind`")
		}
	}

	if paramRef.Name != "" {
		param, err := client.GetResource(ctx, apiVersion, kind, paramsNamespace, paramRef.Name, "")
		if err != nil {
			return nil, err
		}
		return []runtime.Object{param}, nil
	} else if paramRef.Selector != nil {
		paramList, err := client.ListResource(ctx, apiVersion, kind, paramsNamespace, paramRef.Selector)
		if err != nil {
			return nil, err
		}
		for i := range paramList.Items {
			params = append(params, &paramList.Items[i])
		}
	}

	if len(params) == 0 && paramRef.ParameterNotFoundAction != nil && *paramRef.ParameterNotFoundAction == admissionregistrationv1.DenyAction {
		return nil, fmt.Errorf("no params found")
	}

	return params, nil
}
