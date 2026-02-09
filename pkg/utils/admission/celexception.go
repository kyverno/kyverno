package admission

import (
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalCELPolicyException(raw []byte) (*policiesv1beta1.PolicyException, error) {
	var exception *policiesv1beta1.PolicyException
	if err := json.Unmarshal(raw, &exception); err != nil {
		return nil, err
	}
	return exception, nil
}

func GetCELPolicyExceptions(request admissionv1.AdmissionRequest) (*policiesv1beta1.PolicyException, *policiesv1beta1.PolicyException, error) {
	var empty *policiesv1beta1.PolicyException
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
