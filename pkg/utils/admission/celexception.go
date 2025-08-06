package admission

import (
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalCELPolicyException(raw []byte) (*policiesv1alpha1.PolicyException, error) {
	var exception *policiesv1alpha1.PolicyException
	if err := json.Unmarshal(raw, &exception); err != nil {
		return nil, err
	}
	return exception, nil
}

func GetCELPolicyExceptions(request admissionv1.AdmissionRequest) (*policiesv1alpha1.PolicyException, *policiesv1alpha1.PolicyException, error) {
	var empty *policiesv1alpha1.PolicyException
	exception, err := UnmarshalCELPolicyException(request.Object.Raw)
	if err != nil {
		return exception, empty, err
	}
	if request.Operation == admissionv1.Update {
		old, err := UnmarshalCELPolicyException(request.OldObject.Raw)
		return exception, old, err
	}
	return exception, empty, nil
}
