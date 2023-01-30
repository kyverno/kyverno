package engine

import (
	"time"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

// GenerateResponse checks for validity of generate rule on the resource
func GenerateResponse(rclient registryclient.Client, policyContext *PolicyContext, gr kyvernov1beta1.UpdateRequest) (resp *engineapi.EngineResponse) {
	policyStartTime := time.Now()
	return filterGenerateRules(rclient, policyContext, gr.Spec.Policy, policyStartTime)
}

func filterGenerateRules(rclient registryclient.Client, policyContext *PolicyContext, policyNameKey string, startTime time.Time) *engineapi.EngineResponse {
	kind := policyContext.newResource.GetKind()
	name := policyContext.newResource.GetName()
	namespace := policyContext.newResource.GetNamespace()
	apiVersion := policyContext.newResource.GetAPIVersion()

	resp := &engineapi.EngineResponse{
		PolicyResponse: engineapi.PolicyResponse{
			PolicyStats: engineapi.PolicyStats{
				ExecutionStats: engineapi.ExecutionStats{
					Timestamp: startTime.Unix(),
				},
			},
			Resource: engineapi.ResourceSpec{
				Kind:       kind,
				Name:       name,
				Namespace:  namespace,
				APIVersion: apiVersion,
			},
		},
	}

	if policyContext.excludeResourceFunc(kind, namespace, name) {
		logging.WithName("Generate").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	for _, rule := range autogen.ComputeRules(policyContext.policy) {
		if ruleResp := filterRule(rclient, rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}

	return resp
}
