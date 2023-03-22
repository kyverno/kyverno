package engine

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

// GenerateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) engineapi.EngineResponse {
	return e.filterGenerateRules(policyContext, logger, gr.Spec.Policy, time.Now())
}

func (e *engine) filterGenerateRules(
	policyContext engineapi.PolicyContext,
	logger logr.Logger,
	policyNameKey string,
	startTime time.Time,
) engineapi.EngineResponse {
	resp := engineapi.NewEngineResponseFromPolicyContext(policyContext, nil)
	resp.PolicyResponse = engineapi.PolicyResponse{
		Stats: engineapi.PolicyStats{
			ExecutionStats: engineapi.ExecutionStats{
				Timestamp: startTime.Unix(),
			},
		},
	}
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(rule, logger, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}
	return resp
}
