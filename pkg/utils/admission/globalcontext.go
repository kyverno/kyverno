package admission

import (
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalGlobalContextEntry(raw []byte) (*kyvernov2alpha1.GlobalContextEntry, error) {
	var exception *kyvernov2alpha1.GlobalContextEntry
	if err := json.Unmarshal(raw, &exception); err != nil {
		return nil, err
	}
	return exception, nil
}

func GetGlobalContextEntry(request admissionv1.AdmissionRequest) (*kyvernov2alpha1.GlobalContextEntry, *kyvernov2alpha1.GlobalContextEntry, error) {
	var empty *kyvernov2alpha1.GlobalContextEntry
	gctx, err := UnmarshalGlobalContextEntry(request.Object.Raw)
	if err != nil {
		return gctx, empty, err
	}
	if request.Operation == admissionv1.Update {
		old, err := UnmarshalGlobalContextEntry(request.OldObject.Raw)
		return gctx, old, err
	}
	return gctx, empty, nil
}
