package handlers

import (
	"go.opentelemetry.io/otel/attribute"
	admissionv1 "k8s.io/api/admission/v1"
)

func admissionRequestAttributes(request *admissionv1.AdmissionRequest) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("kind", request.Kind.Kind),
		attribute.String("namespace", request.Namespace),
		attribute.String("name", request.Name),
		attribute.String("operation", string(request.Operation)),
		attribute.String("uid", string(request.UID)),
	}
}
