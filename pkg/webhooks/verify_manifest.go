package webhooks

import (
	"fmt"
	"github.com/go-logr/logr"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"k8s.io/api/admission/v1beta1"
)

func (ws *WebhookServer) applyVerifyManifestPolicies(request *v1beta1.AdmissionRequest,
	policyContext *engine.PolicyContext, policies []*v1.ClusterPolicy, logger logr.Logger) error {

	if len(policies) == 0 {
		return nil
	}

	resourceName := getResourceName(request)
	logger = logger.WithValues("action", "verifyManifests", "resource", resourceName)


	var engineResponses []*response.EngineResponse
	for _, p := range policies {
		policyContext.Policy = *p
		resp := engine.VerifyManifestSignature(policyContext, logger)
		engineResponses = append(engineResponses, resp)
	}

	prInfos := policyreport.GeneratePRsFromEngineResponse(engineResponses, logger)
	ws.prGenerator.Add(prInfos...)

	blocked := toBlockResource(engineResponses, logger)

	logger.Info("completed manifest check", "blocked", blocked, "responses", engineResponses)

	if blocked {
		message := getEnforceFailureErrorMsg(engineResponses)
		logger.V(4).Info("resource blocked", "reason", message)
		return fmt.Errorf(message)
	}

	return nil
}

