package admissionpolicy

import (
	"context"
	"fmt"
	"slices"

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
	MutatingAdmissionPolicyVersionV1beta1  MutatingAdmissionPolicyVersion = "v1beta1"
	MutatingAdmissionPolicyVersionV1alpha1 MutatingAdmissionPolicyVersion = "v1alpha1"
)

var (
	errMutatingAdmissionPolicyNotRegistered = fmt.Errorf("mutating admission policy API is not registered")

	supportedMutatingAdmissionPolicyVersions = []MutatingAdmissionPolicyVersion{
		MutatingAdmissionPolicyVersionV1,
		MutatingAdmissionPolicyVersionV1beta1,
		MutatingAdmissionPolicyVersionV1alpha1,
	}
)

func hasPermissions(resource schema.GroupVersionResource, s checker.AuthChecker) bool {
	can, err := checker.Check(
		context.TODO(),
		s,
		resource.Group,
		resource.Version,
		resource.Resource,
		"",
		"",
		"create",
		"update",
		"list",
		"delete",
	)
	if err != nil {
		return false
	}
	return can
}

func HasValidatingAdmissionPolicyPermission(s checker.AuthChecker) bool {
	return hasPermissions(schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1",
		Resource: "validatingadmissionpolicies",
	}, s)
}

func HasValidatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	return hasPermissions(schema.GroupVersionResource{
		Group:    "admissionregistration.k8s.io",
		Version:  "v1",
		Resource: "validatingadmissionpolicybindings",
	}, s)
}

func HasMutatingAdmissionPolicyPermission(s checker.AuthChecker) bool {
	for _, version := range supportedMutatingAdmissionPolicyVersions {
		if hasPermissions(schema.GroupVersionResource{
			Group:    "admissionregistration.k8s.io",
			Version:  string(version),
			Resource: "mutatingadmissionpolicies",
		}, s) {
			return true
		}
	}
	return false
}

func HasMutatingAdmissionPolicyBindingPermission(s checker.AuthChecker) bool {
	for _, version := range supportedMutatingAdmissionPolicyVersions {
		if hasPermissions(schema.GroupVersionResource{
			Group:    "admissionregistration.k8s.io",
			Version:  string(version),
			Resource: "mutatingadmissionpolicybindings",
		}, s) {
			return true
		}
	}
	return false
}

func isRegistered(
	kubeClient kubernetes.Interface,
	group,
	version string,
	resources ...string,
) (bool, error) {

	resourceList, err := kubeClient.Discovery().
		ServerResourcesForGroupVersion(schema.GroupVersion{
			Group:   group,
			Version: version,
		}.String())

	if err != nil {
		return false, err
	}

	available := make([]string, 0, len(resourceList.APIResources))
	for _, r := range resourceList.APIResources {
		available = append(available, r.Name)
	}

	for _, r := range resources {
		if !slices.Contains(available, r) {
			return false, nil
		}
	}

	return true, nil
}

func PreferredMutatingAdmissionPolicyVersion(kubeClient kubernetes.Interface) (MutatingAdmissionPolicyVersion, error) {
	for _, version := range supportedMutatingAdmissionPolicyVersions {
		registered, err := isRegistered(
			kubeClient,
			"admissionregistration.k8s.io",
			string(version),
			"mutatingadmissionpolicies",
			"mutatingadmissionpolicybindings",
		)

		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return "", err
		}

		if registered {
			return version, nil
		}
	}

	return "", errMutatingAdmissionPolicyNotRegistered
}

func IsMutatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	_, err := PreferredMutatingAdmissionPolicyVersion(kubeClient)
	if err != nil {
		if err == errMutatingAdmissionPolicyNotRegistered {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func IsValidatingAdmissionPolicyRegistered(kubeClient kubernetes.Interface) (bool, error) {
	registered, err := isRegistered(
		kubeClient,
		"admissionregistration.k8s.io",
		"v1",
		"validatingadmissionpolicies",
		"validatingadmissionpolicybindings",
	)

	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return registered, nil
}

func CollectParams(
	ctx context.Context,
	client engineapi.Client,
	paramKind *admissionregistrationv1.ParamKind,
	paramRef *admissionregistrationv1.ParamRef,
	namespace string,
) ([]runtime.Object, error) {

	var params []runtime.Object

	gv, err := schema.ParseGroupVersion(paramKind.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid paramKind apiVersion: %w", err)
	}

	isNamespaced, err := client.IsNamespaced(gv.Group, gv.Version, paramKind.Kind)
	if err != nil {
		return nil, fmt.Errorf("failed to check namespaced resource: %w", err)
	}

	paramsNamespace := ""
	if isNamespaced {
		if paramRef.Namespace != "" {
			paramsNamespace = paramRef.Namespace
		} else {
			paramsNamespace = namespace
		}

		if paramsNamespace == "" {
			return nil, fmt.Errorf("cannot resolve namespace for namespaced paramKind")
		}
	} else {
		if paramRef.Namespace != "" {
			return nil, fmt.Errorf("paramRef.namespace not allowed for cluster-scoped paramKind")
		}
	}

	if paramRef.Name != "" {
		obj, err := client.GetResource(ctx, paramKind.APIVersion, paramKind.Kind, paramsNamespace, paramRef.Name, "")
		if err != nil {
			return nil, err
		}
		return []runtime.Object{obj}, nil
	}

	if paramRef.Selector != nil {
		list, err := client.ListResource(ctx, paramKind.APIVersion, paramKind.Kind, paramsNamespace, paramRef.Selector)
		if err != nil {
			return nil, err
		}

		for i := range list.Items {
			params = append(params, &list.Items[i])
		}
	}

	if len(params) == 0 &&
		paramRef.ParameterNotFoundAction != nil &&
		*paramRef.ParameterNotFoundAction == admissionregistrationv1.DenyAction {
		return nil, fmt.Errorf("no params found")
	}

	return params, nil
}
