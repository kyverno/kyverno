package handlers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	admissionv1 "k8s.io/api/admission/v1"
)

// WithLegacyPolicyDenial denies every create/update of a legacy kyverno.io policy kind, except
// delete/connect and a narrow finalizer-removal recovery update. It must be chained outside
// WithSubResourceFilter(): that filter returns success for every subresource request before the
// inner handler runs, so a gate placed inside it would never see a subresource write and would
// silently reinstate the blanket subresource bypass this decorator exists to remove.
func (inner AdmissionHandler) WithLegacyPolicyDenial() AdmissionHandler {
	return inner.withLegacyPolicyDenial(deprecations.DenyLegacyWrite).WithTrace("LEGACY")
}

func (inner AdmissionHandler) withLegacyPolicyDenial(decide func(admissionv1.AdmissionRequest) (error, bool)) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		if err, denied := decide(request.AdmissionRequest); denied {
			logger.Error(err, "legacy policy write denied", "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
			if deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics(); deprecatedMetric != nil {
				deprecatedMetric.Record(ctx, request.Namespace, request.Kind.Group, request.Kind.Version, request.Kind.Kind, "")
			}
			return admissionutils.Response(request.UID, err)
		}
		return inner(ctx, logger, request, startTime)
	}
}
