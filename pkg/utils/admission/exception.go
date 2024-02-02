package admission

import (
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalPolicyException(raw []byte) (*kyvernov2beta1.PolicyException, error) {
	var exception *kyvernov2beta1.PolicyException
	if err := json.Unmarshal(raw, &exception); err != nil {
		return nil, err
	}
	return exception, nil
}

func GetPolicyExceptions(request admissionv1.AdmissionRequest) (*kyvernov2beta1.PolicyException, *kyvernov2beta1.PolicyException, error) {
	var empty *kyvernov2beta1.PolicyException
	exception, err := UnmarshalPolicyException(request.Object.Raw)
	if err != nil {
		return exception, empty, err
	}
	if request.Operation == admissionv1.Update {
		old, err := UnmarshalPolicyException(request.OldObject.Raw)
		return exception, old, err
	}
	return exception, empty, nil
}
