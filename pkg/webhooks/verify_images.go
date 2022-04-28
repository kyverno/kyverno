package webhooks

import (
	"errors"

	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policyreport"
	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	jsonutils "github.com/kyverno/kyverno/pkg/utils/json"
	admissionv1 "k8s.io/api/admission/v1"
)

func (ws *WebhookServer) applyImageVerifyPolicies(request *admissionv1.AdmissionRequest, policyContext *engine.PolicyContext, policies []v1.PolicyInterface, logger logr.Logger) ([]byte, error) {
	ok, message, imagePatches := ws.handleVerifyImages(request, policyContext, policies)
	if !ok {
		return nil, errors.New(message)
	}

	logger.V(6).Info("images verified", "patches", string(imagePatches))
	return imagePatches, nil
}

func (ws *WebhookServer) handleVerifyImages(request *admissionv1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []v1.PolicyInterface) (bool, string, []byte) {

	if len(policies) == 0 {
		return true, "", nil
	}

	resourceName := admissionutils.GetResourceName(request)
	logger := ws.log.WithValues("action", "verifyImages", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var engineResponses []*response.EngineResponse
	var patches [][]byte
	for _, p := range policies {
		policyContext.Policy = p
		resp := engine.VerifyAndPatchImages(policyContext)
		engineResponses = append(engineResponses, resp)
		patches = append(patches, resp.GetPatches()...)
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	ws.prGenerator.Add(prInfos...)

	blocked := toBlockResource(engineResponses, logger)

	events := generateEvents(engineResponses, blocked, logger)
	ws.eventGen.Add(events...)

	if blocked {
		logger.V(4).Info("resource blocked")
		return false, getEnforceFailureErrorMsg(engineResponses), nil
	}

	return true, "", jsonutils.JoinPatches(patches...)
}
