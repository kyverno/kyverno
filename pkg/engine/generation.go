package engine

import (
	"context"
	"time"

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
	for _, rule := range autogen.Default.ComputeRules(policyContext.Policy(), "") {
		startTime := time.Now()
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(ctx, rule, logger, policyContext); ruleResp != nil {
			r := *ruleResp
			resp.Rules = append(resp.Rules, r.WithStats(engineapi.NewExecutionStats(startTime, time.Now())))
		}
	}
	return resp
}
