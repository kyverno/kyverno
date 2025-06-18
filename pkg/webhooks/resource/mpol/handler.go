package mpol

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/julienschmidt/httprouter"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
)

type handler struct {
	context libs.Context
	engine  mpolengine.Engine
}

func New(
	context libs.Context,
	engine mpolengine.Engine,
) *handler {
	return &handler{
		context: context,
		engine:  engine,
	}
}

func (h *handler) Mutate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, _ string, _ time.Time) handlers.AdmissionResponse {
	var policies []string
	if params := httprouter.ParamsFromContext(ctx); params != nil {
		if params := strings.Split(strings.TrimLeft(params.ByName("policies"), "/"), "/"); len(params) != 0 {
			policies = params
		}
	}

	if len(policies) == 0 {
		return admissionutils.ResponseSuccess(admissionRequest.UID)
	}

	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.Handle(ctx, request, mpolengine.MatchNames(policies...))
	if err != nil {
		logger.Error(err, "failed to handle mutating policy request")
		return admissionutils.Response(admissionRequest.UID, err)
	}

	return h.admissionResponse(request, response)
}

func (h *handler) admissionResponse(request celengine.EngineRequest, response mpolengine.EngineResponse) handlers.AdmissionResponse {
	if len(response.Policies) == 0 {
		return admissionutils.ResponseSuccess(request.Request.UID)
	}

	var errs, warnings []string
	for _, policy := range response.Policies {
		for _, rule := range policy.Rules {
			if rule.Status() == engineapi.RuleStatusError {
				errs = append(errs, rule.Message())
			} else if rule.Status() == engineapi.RuleStatusWarn {
				warnings = append(warnings, rule.Message())
			}
		}
	}

	if response.PatchedResource != nil {
		patches := jsonutils.JoinPatches(patch.ConvertPatches(response.GetPatches()...)...)
		return admissionutils.MutationResponse(request.Request.UID, patches, warnings...)

	}

	return admissionutils.MutationResponse(request.Request.UID, nil, warnings...)
}
