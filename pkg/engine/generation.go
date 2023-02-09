package engine

import (
	"context"
	"time"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/logging"
	"k8s.io/client-go/tools/cache"
)

// generateResponse checks for validity of generate rule on the resource
func (e *engine) generateResponse(
	ctx context.Context,
	policyContext engineapi.PolicyContext,
	gr kyvernov1beta1.UpdateRequest,
) *engineapi.PolicyResponse {
	startTime := time.Now()
	newResource := policyContext.NewResource()
	kind := newResource.GetKind()
	name := newResource.GetName()
	namespace := newResource.GetNamespace()
	apiVersion := newResource.GetAPIVersion()
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(gr.Spec.Policy)
	if err != nil {
		logging.Error(err, "failed to spilt name and namespace", gr.Spec.Policy)
	}
	resp := &engineapi.PolicyResponse{
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
	}
	if e.configuration.ToFilter(kind, namespace, name) {
		logging.WithName("Generate").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	for _, rule := range autogen.ComputeRules(policyContext.Policy()) {
		if ruleResp := e.filterRule(rule, policyContext); ruleResp != nil {
			resp.Rules = append(resp.Rules, *ruleResp)
		}
	}

	return resp
}
