package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/config"
	webhookutils "github.com/kyverno/kyverno/pkg/webhooks/utils"
	admissionv1 "k8s.io/api/admission/v1"
)

func (inner AdmissionHandler) WithFilter(configuration config.Configuration) AdmissionHandler {
	return inner.withFilter(configuration).WithTrace("FILTER")
}

func (inner AdmissionHandler) withFilter(c config.Configuration) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request *admissionv1.AdmissionRequest, startTime time.Time) *admissionv1.AdmissionResponse {
		if c.ToFilter(request.Kind.Kind, request.Namespace, request.Name) {
			return nil
		}
		if webhookutils.ExcludeKyvernoResources(request.Kind.Kind) {
			return nil
		}
		return inner(ctx, logger, request, startTime)
	}
}
