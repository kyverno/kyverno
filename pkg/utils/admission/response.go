package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var patchTypeJSONPatch = admissionv1.PatchTypeJSONPatch

func Response(uid types.UID, err error, warnings ...string) admissionv1.AdmissionResponse {
	response := admissionv1.AdmissionResponse{
		Allowed: err == nil,
		UID:     uid,
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

func ResponseSuccess(uid types.UID, warnings ...string) admissionv1.AdmissionResponse {
	return Response(uid, nil, warnings...)
}

func MutationResponse(uid types.UID, patch []byte, warnings ...string) admissionv1.AdmissionResponse {
	response := ResponseSuccess(uid, warnings...)
	if len(patch) != 0 {
		response.Patch = patch
		response.PatchType = &patchTypeJSONPatch
	}
	return response
}
