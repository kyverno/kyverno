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
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
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

	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, err := h.engine.Handle(ctx, request, mpolengine.MatchNames(policies...))
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}

	return h.admissionResponse(request, response)
}

func (h *handler) admissionResponse(request celengine.EngineRequest, response mpolengine.EngineResponse) handlers.AdmissionResponse {
	return handlers.AdmissionResponse{}
}
