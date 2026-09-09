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
	apiequality "k8s.io/apimachinery/pkg/api/equality"
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
	policy, oldPolicy, err := admissionutils.GetCleanupPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}
	if err, blocked := deprecations.ShouldBlock(ctx, request.AdmissionRequest, func() bool {
		return oldPolicy != nil && apiequality.Semantic.DeepEqual(oldPolicy.GetSpec(), policy.GetSpec())
	}); blocked {
		logger.Error(err, "legacy policy write blocked", "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
		if deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics(); deprecatedMetric != nil {
			deprecatedMetric.Record(ctx, request.Namespace, request.Kind.Group, request.Kind.Version, request.Kind.Kind, "")
		}
		return admissionutils.Response(request.UID, err)
	}
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
