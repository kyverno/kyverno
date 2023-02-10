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

// generateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	logger logr.Logger,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) *engineapi.PolicyResponse {
	startTime := time.Now()
	newResource := policyContext.NewResource()
	kind := newResource.GetKind()
	name := newResource.GetName()
	namespace := newResource.GetNamespace()
	apiVersion := newResource.GetAPIVersion()
	resp := &engineapi.PolicyResponse{
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
		logger.Info("resource excluded")
		return resp
	}
	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		logger := internal.LoggerWithRule(logger, rule)
		if ruleResp := e.filterRule(rule, logger, policyContext); ruleResp != nil {
			resp.Rules = append(resp.Rules, *ruleResp)
		}
	}
	return resp
}
