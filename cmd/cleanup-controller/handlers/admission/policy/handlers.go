package policy

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/deprecations"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	validation "github.com/kyverno/kyverno/pkg/validation/cleanuppolicy"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type validationHandlers struct {
	client dclient.Interface
}

func New(client dclient.Interface) *validationHandlers {
	return &validationHandlers{
		client: client,
	}
}

func (h *validationHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ time.Time) handlers.AdmissionResponse {
	// Subresource requests (e.g. /status, written by this controller's own reconcile loop)
	// never touch spec, so there is nothing here to validate or warn about; short-circuit
	// before policy validation and deprecation warnings, not just the legacy-policy block.
	// Kubernetes guarantees a status-subresource write cannot change spec, see:
	// https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#status-subresource
	if request.SubResource != "" {
		return admissionutils.ResponseSuccess(request.UID)
	}

	policy, _, err := admissionutils.GetCleanupPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	// The legacy write denial (create/update of this kind) now happens one layer up, at the
	// route-level handlers.WithLegacyPolicyDenial() decorator (see
	// pkg/deprecations.DenyLegacyWrite), so this method only runs on requests it already
	// allowed through.
	if err := validation.Validate(ctx, logger, h.client, policy); err != nil {
		logger.Error(err, "policy validation errors")
		return admissionutils.Response(request.UID, err)
	}
	if warning, ok := deprecations.BuildKindWarning(request.Kind.Group, request.Kind.Version, request.Kind.Kind); ok {
		logger.V(2).Info(warning.Message, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
		if deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics(); deprecatedMetric != nil {
			deprecatedMetric.Record(ctx, request.Namespace, warning.Group, warning.Version, warning.Kind, "")
		}
		return admissionutils.Response(request.UID, nil, warning.Message)
	}
	return admissionutils.ResponseSuccess(request.UID)
}
