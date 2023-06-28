package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
)

func IsDryRun(request admissionv1.AdmissionRequest) bool {
	return request.DryRun != nil && *request.DryRun
}
