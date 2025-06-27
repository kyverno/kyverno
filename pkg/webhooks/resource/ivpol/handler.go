package ivpol

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	ivpolengine "github.com/kyverno/kyverno/pkg/cel/policies/ivpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	"go.uber.org/multierr"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type handler struct {
	context libs.Context
	engine  ivpolengine.Engine
}

func New(
	engine ivpolengine.Engine,
	context libs.Context,
) *handler {
	return &handler{
		context: context,
		engine:  engine,
	}
}

func (h *handler) Mutate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, patches, err := h.engine.HandleMutating(ctx, request, ivpolengine.MatchNames(policies...))
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	rawPatches := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
	return h.mutationResponse(request, response, rawPatches)
}

func (h *handler) Validate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.HandleValidating(ctx, request, ivpolengine.MatchNames(policies...))
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	return h.validationResponse(request, response)
}

func (h *handler) mutationResponse(request celengine.EngineRequest, response eval.ImageVerifyEngineResponse, rawPatches []byte) handlers.AdmissionResponse {
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Warn) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				warnings = append(warnings, fmt.Sprintf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				warnings = append(warnings, fmt.Sprintf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
	}
	return admissionutils.MutationResponse(request.AdmissionRequest().UID, rawPatches, warnings...)
}

func (h *handler) validationResponse(request celengine.EngineRequest, response eval.ImageVerifyEngineResponse) handlers.AdmissionResponse {
	var errs []error
	var warnings []string
	for _, policy := range response.Policies {
		if policy.Actions.Has(admissionregistrationv1.Deny) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				errs = append(errs, fmt.Errorf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				errs = append(errs, fmt.Errorf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
		if policy.Actions.Has(admissionregistrationv1.Warn) {
			switch policy.Result.Status() {
			case engineapi.RuleStatusFail:
				warnings = append(warnings, fmt.Sprintf("Policy %s failed: %s", policy.Policy.GetName(), policy.Result.Message()))
			case engineapi.RuleStatusError:
				warnings = append(warnings, fmt.Sprintf("Policy %s error: %s", policy.Policy.GetName(), policy.Result.Message()))
			}
		}
	}
	return admissionutils.Response(request.AdmissionRequest().UID, multierr.Combine(errs...), warnings...)
}
