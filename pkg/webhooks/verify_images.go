package webhooks

import (
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) handleVerifyImages(request *v1beta1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []*v1.ClusterPolicy) (bool, string) {

	if len(policies) == 0 {
		return true, ""
	}

	resourceName := getResourceName(request)
	logger := ws.log.WithValues("action", "verifyImages", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var engineResponses []*response.EngineResponse
	for _, p := range policies {
		policyContext.Policy = *p
		resp := engine.VerifyImages(policyContext)
		engineResponses = append(engineResponses, resp)
	}

	blocked := toBlockResource(engineResponses, logger)
	if blocked {
		logger.V(4).Info("resource blocked")
		return false, getEnforceFailureErrorMsg(engineResponses)
	}

	return true, ""
}
