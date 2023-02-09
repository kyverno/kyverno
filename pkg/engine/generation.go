package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
	"k8s.io/client-go/tools/cache"
)

// GenerateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) (resp *engineapi.EngineResponse) {
	return e.filterGenerateRules(policyContext, logger, gr.Spec.Policy, time.Now())
}

func (e *engine) filterGenerateRules(
	policyContext engineapi.PolicyContext,
	logger logr.Logger,
	policyNameKey string,
	startTime time.Time,
) *engineapi.EngineResponse {
	newResource := policyContext.NewResource()
	kind := newResource.GetKind()
	name := newResource.GetName()
	namespace := newResource.GetNamespace()
	apiVersion := newResource.GetAPIVersion()
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(policyNameKey)
	if err != nil {
		logger.Error(err, "failed to spilt name and namespace", "policy.key", policyNameKey)
	}
	resp := &engineapi.EngineResponse{
		PolicyResponse: engineapi.PolicyResponse{
			Policy: engineapi.PolicySpec{
				Name:      pName,
				Namespace: pNamespace,
			},
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
	if !internal.MatchPolicyContext(logger, policyContext, e.configuration) {
		return resp
	}
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(rule, logger, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}
