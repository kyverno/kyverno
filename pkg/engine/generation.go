package engine

import (
	"context"
	"time"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
)

// GenerateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) (resp *engineapi.EngineResponse) {
	policyStartTime := time.Now()
	return e.filterGenerateRules(policyContext, gr.Spec.Policy, policyStartTime)
}

func (e *engine) filterGenerateRules(
	policyContext engineapi.PolicyContext,
	policyNameKey string,
	startTime time.Time,
) *engineapi.EngineResponse {
	newResource := policyContext.NewResource()
	kind := newResource.GetKind()
	name := newResource.GetName()
	namespace := newResource.GetNamespace()
	apiVersion := newResource.GetAPIVersion()
	resp := engineapi.NewEngineResponse(policyContext.Policy())
	resp.PolicyResponse = engineapi.PolicyResponse{
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
	}
	if e.configuration.ToFilter(kind, namespace, name) {
		logging.WithName("Generate").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		if ruleResp := e.filterRule(rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}

	return resp
}
