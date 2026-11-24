package metrics

import (
	"fmt"
	"slices"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GetResourceKindAndNamespace safely extracts the kind and namespace from a possibly nil resource.
func GetResourceKindAndNamespace(resource *unstructured.Unstructured) (string, string) {
	if resource == nil {
		return "", ""
	}
	return resource.GetKind(), resource.GetNamespace()
}

type GenericPolicy interface {
	metav1.Object
	GetMatchConstraints() admissionregistrationv1.MatchResources
	GetMatchConditions() []admissionregistrationv1.MatchCondition
	GetFailurePolicy(bool) admissionregistrationv1.FailurePolicyType
	GetVariables() []admissionregistrationv1.Variable
}

func ParseResourceRequestOperation(requestOperationStr string) (ResourceRequestOperation, error) {
	switch requestOperationStr {
	case "CREATE":
		return ResourceCreated, nil
	case "UPDATE":
		return ResourceUpdated, nil
	case "DELETE":
		return ResourceDeleted, nil
	case "CONNECT":
		return ResourceConnected, nil
	default:
		return "", fmt.Errorf("unknown request operation made by resource: %s. Allowed requests: 'CREATE', 'UPDATE', 'DELETE', 'CONNECT'", requestOperationStr)
	}
}

func GetCELPolicyInfos(policy GenericPolicy) (string, string, PolicyBackgroundMode, admissionregistrationv1.ValidationAction) {
	name := policy.GetName()
	validationMode := admissionregistrationv1.Audit
	backgroundMode := BackgroundFalse
	policyType := ""

	switch p := policy.(type) {
	case v1beta1.ValidatingPolicyLike:
		policyType = "Validating"

		if p.GetSpec().BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
		if slices.Contains(p.GetSpec().ValidationActions(), admissionregistrationv1.Deny) {
			validationMode = admissionregistrationv1.Deny
		}
	case v1beta1.ImageValidatingPolicyLike:
		policyType = "ImageValidating"

		if p.GetSpec().BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
		if slices.Contains(p.GetSpec().ValidationActions(), admissionregistrationv1.Deny) {
			validationMode = admissionregistrationv1.Deny
		}
	case v1beta1.MutatingPolicyLike:
		policyType = "Mutating"

		if p.GetSpec().BackgroundEnabled() {
			backgroundMode = BackgroundTrue
		}
	case v1beta1.GeneratingPolicyLike:
		policyType = "Generating"
	}

	return name, policyType, backgroundMode, validationMode
}
