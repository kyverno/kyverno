package policy

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	dpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/dpol"
	gpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/gpol"
	mpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/mpol"
	vpolvalidation "github.com/kyverno/kyverno/pkg/cel/policies/vpol"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/deprecations"
	eval "github.com/kyverno/kyverno/pkg/image/verification/evaluator"
	"github.com/kyverno/kyverno/pkg/metrics"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	policyvalidate "github.com/kyverno/kyverno/pkg/validation/policy"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type policyHandlers struct {
	client                       dclient.Interface
	secretLister                 corev1listers.SecretLister
	backgroundServiceAccountName string
	reportsServiceAccountName    string
}

func NewHandlers(client dclient.Interface, secretLister corev1listers.SecretLister, backgroundSA, reportsSA string) *policyHandlers {
	return &policyHandlers{
		secretLister:                 secretLister,
		client:                       client,
		backgroundServiceAccountName: backgroundSA,
		reportsServiceAccountName:    reportsSA,
	}
}

func (h *policyHandlers) Validate(ctx context.Context, logger logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	policy, oldPolicy, err := admissionutils.GetPolicies(request.AdmissionRequest)
	if err != nil {
		logger.Error(err, "failed to unmarshal policies from admission request")
		return admissionutils.Response(request.UID, err)
	}

	if vpol := policy.AsValidatingPolicyLike(); vpol != nil {
		warnings, err := vpolvalidation.Validate(vpol)
		if err != nil {
			logger.Error(err, "ValidatingPolicy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if ivpol := policy.AsImageValidatingPolicyLike(); ivpol != nil {
		warnings, err := eval.Validate(ivpol, h.secretLister)
		if err != nil {
			logger.Error(err, "ImageValidatingPolicy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if mpol := policy.AsMutatingPolicyLike(); mpol != nil {
		warnings, err := mpolvalidation.Validate(mpol)
		if err != nil {
			logger.Error(err, "MutatingPolicy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if gpol := policy.AsGeneratingPolicyLike(); gpol != nil {
		warnings, err := gpolvalidation.Validate(gpol)
		if err != nil {
			logger.Error(err, "GeneratingPolicy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if dpol := policy.AsDeletingPolicy(); dpol != nil {
		warnings, err := dpolvalidation.Validate(dpol)
		if err != nil {
			logger.Error(err, "DeletingPolicy validation errors")
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	if pol := policy.AsKyvernoPolicy(); pol != nil {
		var old kyvernov1.PolicyInterface
		if oldPolicy != nil {
			old = oldPolicy.AsKyvernoPolicy()
		}

		warnings, err := policyvalidate.Validate(policy.AsKyvernoPolicy(), old, h.client, false, h.backgroundServiceAccountName, h.reportsServiceAccountName)
		if err != nil {
			logger.Error(err, "policy validation errors")
		}
		deprecatedMetric := metrics.GetDeprecatedAPIRequestMetrics()
		if warning, ok := deprecations.BuildKindWarning(request.Kind.Group, request.Kind.Version, request.Kind.Kind); ok {
			logger.V(2).Info(warning.Message, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
			warnings = append(warnings, warning.Message)
			if deprecatedMetric != nil {
				deprecatedMetric.Record(ctx, request.Namespace, warning.Group, warning.Version, warning.Kind, "")
			}
		}
		for _, warning := range deprecations.PolicyFieldWarnings(pol) {
			logger.V(2).Info(warning.Message, "field", warning.Field, "kind", request.Kind.Kind, "namespace", request.Namespace, "name", request.Name)
			warnings = append(warnings, warning.Message)
			if deprecatedMetric != nil {
				deprecatedMetric.Record(ctx, request.Namespace, warning.Group, warning.Version, warning.Kind, deprecations.NormalizeFieldPath(warning.Field))
			}
		}
		return admissionutils.Response(request.UID, err, warnings...)
	}

	return admissionutils.Response(request.UID, errors.New("failed to convert policy"))
}

func (h *policyHandlers) Mutate(_ context.Context, _ logr.Logger, request handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	return admissionutils.ResponseSuccess(request.UID)
}
