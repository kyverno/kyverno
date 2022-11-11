package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Response(err error, warnings ...string) *admissionv1.AdmissionResponse {
	response := &admissionv1.AdmissionResponse{
		Allowed: err == nil,
	}
	if err != nil {
		response.Result = &metav1.Status{
			Status:  metav1.StatusFailure,
			Message: err.Error(),
		}
	}
	response.Warnings = warnings
	return response
}

func ResponseSuccess(warnings ...string) *admissionv1.AdmissionResponse {
	return Response(nil, warnings...)
}

func MutationResponse(patch []byte, warnings ...string) *admissionv1.AdmissionResponse {
	response := ResponseSuccess(warnings...)
	response.Patch = patch
	return response
}
