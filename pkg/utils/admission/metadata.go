package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
)

func UnmarshalPartialObjectMetadata(raw []byte) (*metav1.PartialObjectMetadata, error) {
	var object *metav1.PartialObjectMetadata
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, err
	}
	return object, nil
}

func GetPartialObjectMetadatas(request admissionv1.AdmissionRequest) (*metav1.PartialObjectMetadata, *metav1.PartialObjectMetadata, error) {
	object, err := UnmarshalPartialObjectMetadata(request.Object.Raw)
	if err != nil {
		return nil, nil, err
	}
	if request.Operation != admissionv1.Update {
		return object, nil, nil
	}
	oldObject, err := UnmarshalPartialObjectMetadata(request.OldObject.Raw)
	if err != nil {
		return nil, nil, err
	}
	return object, oldObject, nil
}
