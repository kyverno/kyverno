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

// WithLegacyPolicyDenial denies every create/update of a legacy kyverno.io policy kind. Delete
// and connect delegate to the inner handler; the narrow finalizer-removal recovery update is
// answered success here without delegating, so it cannot be refused by legacy spec validation. It must be chained outside
// WithSubResourceFilter(): that filter returns success for every subresource request before the
// inner handler runs, so a gate placed inside it would never see a subresource write and would
// silently reinstate the blanket subresource bypass this decorator exists to remove.
func (inner AdmissionHandler) WithLegacyPolicyDenial() AdmissionHandler {
	return inner.withLegacyPolicyDenial(deprecations.DecideLegacyWrite).WithTrace("LEGACY")
}

func (inner AdmissionHandler) withLegacyPolicyDenial(decide func(admissionv1.AdmissionRequest) (deprecations.Decision, error)) AdmissionHandler {
	return func(ctx context.Context, logger logr.Logger, request AdmissionRequest, startTime time.Time) AdmissionResponse {
		switch decision, err := decide(request.AdmissionRequest); decision {
		case deprecations.Deny:
			logger.Error(err, "legacy policy write denied", "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
			if deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics(); deprecatedMetric != nil {
				deprecatedMetric.Record(ctx, request.Namespace, request.Kind.Group, request.Kind.Version, request.Kind.Kind, "")
			}
			return admissionutils.Response(request.UID, err)
		case deprecations.AllowRecovery:
			// Answer success here rather than delegating: the inner handler is the typed
			// legacy validator, which would re-validate the unchanged spec and can refuse a
			// policy that no longer passes current validation, blocking recovery for the
			// stale objects most likely to be stuck. It also goes away with the legacy types.
			logger.V(2).Info("allowing legacy finalizer-removal recovery", "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
			return admissionutils.ResponseSuccess(request.UID)
		}
		return inner(ctx, logger, request, startTime)
	}
}
