package webhooks

import (
	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	engineutils "github.com/kyverno/kyverno/pkg/engine/utils"
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) handleVerifyImages(request *v1beta1.AdmissionRequest,
	policyContext *engine.PolicyContext,
	policies []*v1.ClusterPolicy) (bool, string, []byte) {

	if len(policies) == 0 {
		return true, "", nil
	}

	resourceName := getResourceName(request)
	logger := ws.log.WithValues("action", "verifyImages", "resource", resourceName, "operation", request.Operation, "gvk", request.Kind.String())

	var engineResponses []*response.EngineResponse
	var patches [][]byte
	for _, p := range policies {
		policyContext.Policy = *p
		resp := engine.VerifyAndPatchImages(policyContext)
		engineResponses = append(engineResponses, resp)
		patches = append(patches, resp.GetPatches()...)
	}

	blocked := toBlockResource(engineResponses, logger)
	if blocked {
		logger.V(4).Info("resource blocked")
		return false, getEnforceFailureErrorMsg(engineResponses), nil
	}

	return true, "", engineutils.JoinPatches(patches)
}
