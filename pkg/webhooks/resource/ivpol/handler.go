package ivpol

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	celpolicy "github.com/kyverno/kyverno/pkg/cel/policy"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/mutate/patch"
	eval "github.com/kyverno/kyverno/pkg/imageverification/evaluator"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	"github.com/kyverno/kyverno/pkg/webhooks/handlers"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type handler struct {
	context celpolicy.Context
	engine  celengine.ImageVerifyEngine
}

func New(
	engine celengine.ImageVerifyEngine,
	context celpolicy.Context,
) *handler {
	return &handler{
		context: context,
		engine:  engine,
	}
}

func (h *handler) Mutate(ctx context.Context, logger logr.Logger, admissionRequest handlers.AdmissionRequest, failurePolicy string, startTime time.Time) handlers.AdmissionResponse {
	request := celengine.RequestFromAdmission(h.context, admissionRequest.AdmissionRequest)
	response, patches, err := h.engine.HandleMutating(ctx, request)
	if err != nil {
		return admissionutils.Response(admissionRequest.UID, err)
	}
	rawPatches := jsonutils.JoinPatches(patch.ConvertPatches(patches...)...)
	return h.mutationResponse(request, response, rawPatches)
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
