package engine

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/internal"
)

// GenerateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
) engineapi.PolicyResponse {
	resp := engineapi.NewPolicyResponse()
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(rule, logger, policyContext); ruleResp != nil {
			resp.Rules = append(resp.Rules, *ruleResp)
		}
	}
	return resp
}
