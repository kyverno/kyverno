package admission

import (
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalCELPolicyException(raw []byte) (*kyvernov2alpha1.CELPolicyException, error) {
	var exception *kyvernov2alpha1.CELPolicyException
	if err := json.Unmarshal(raw, &exception); err != nil {
		return nil, err
	}
	return exception, nil
}

func GetCELPolicyExceptions(request admissionv1.AdmissionRequest) (*kyvernov2alpha1.CELPolicyException, *kyvernov2alpha1.CELPolicyException, error) {
	var empty *kyvernov2alpha1.CELPolicyException
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
