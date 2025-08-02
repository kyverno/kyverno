package admissionpolicy

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/auth/checker"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// Collect params collects parameter resources from a live cluster
func CollectParams(ctx context.Context, client engineapi.Client, paramKind *admissionregistrationv1.ParamKind, paramRef *admissionregistrationv1.ParamRef, namespace string) ([]runtime.Object, error) {
	var params []runtime.Object

	apiVersion := paramKind.APIVersion
	kind := paramKind.Kind
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, fmt.Errorf("can't parse the parameter resource group version")
	}

	// If `paramKind` is cluster-scoped, then paramRef.namespace MUST be unset.
	// If `paramKind` is namespace-scoped, the namespace of the object being evaluated for admission will be used
	// when paramRef.namespace is left unset.
	var paramsNamespace string
	isNamespaced, err := client.IsNamespaced(gv.Group, gv.Version, kind)
	if err != nil {
		return nil, fmt.Errorf("failed to check if resource is namespaced or not (%w)", err)
	}

	// check if `paramKind` is namespace-scoped
	if isNamespaced {
		// set params namespace to the incoming object's namespace by default.
		paramsNamespace = namespace
		if paramRef.Namespace != "" {
			paramsNamespace = paramRef.Namespace
		} else if paramsNamespace == "" {
			return nil, fmt.Errorf("can't use namespaced paramRef to match cluster-scoped resources")
		}
	} else {
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
