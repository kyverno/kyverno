package handlers

import (
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	admissionv1 "k8s.io/api/admission/v1"
)

func (h AdmissionHandler) WithFilter(configuration config.Configuration) AdmissionHandler {
	return withFilter(configuration, h)
}

func withFilter(c config.Configuration, inner AdmissionHandler) AdmissionHandler {
	return func(logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			return nil
		}
		return inner(logger, request, startTime)
	}
}
