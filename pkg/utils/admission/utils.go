package admission

import (
	admissionv1 "k8s.io/api/admission/v1"
)

func GetResourceName(request *admissionv1.AdmissionRequest) string {
	resourceName := request.Kind.Kind + "/" + request.Name
	if request.Namespace != "" {
		resourceName = request.Namespace + "/" + resourceName
	}
	return resourceName
}
