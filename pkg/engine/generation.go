package engine

import (
	"time"

	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// GenerateResponse checks for validity of generate rule on the resource
func GenerateResponse(policyContext *PolicyContext, gr urkyverno.UpdateRequest) (resp *response.EngineResponse) {
	policyStartTime := time.Now()
	return filterGenerateRules(policyContext, gr.Spec.Policy, policyStartTime)
}

func filterGenerateRules(policyContext *PolicyContext, policyNameKey string, startTime time.Time) *response.EngineResponse {
	kind := policyContext.NewResource.GetKind()
	name := policyContext.NewResource.GetName()
	namespace := policyContext.NewResource.GetNamespace()
	apiVersion := policyContext.NewResource.GetAPIVersion()
	pNamespace, pName, err := cache.SplitMetaNamespaceKey(policyNameKey)
	if err != nil {
		log.Log.Error(err, "failed to spilt name and namespace", policyNameKey)
	}

	resp := &response.EngineResponse{
		PolicyResponse: response.PolicyResponse{
			Policy: response.PolicySpec{
				Name:      pName,
				Namespace: pNamespace,
			},
			PolicyStats: response.PolicyStats{
				PolicyExecutionTimestamp: startTime.Unix(),
			},
			Resource: response.ResourceSpec{
				Kind:       kind,
				Name:       name,
				Namespace:  namespace,
				APIVersion: apiVersion,
			},
		},
	}

	if policyContext.ExcludeResourceFunc(kind, namespace, name) {
		log.Log.WithName("Generate").Info("resource excluded", "kind", kind, "namespace", namespace, "name", name)
		return resp
	}

	for _, rule := range autogen.ComputeRules(policyContext.Policy) {
		if ruleResp := filterRule(rule, policyContext); ruleResp != nil {
			resp.PolicyResponse.Rules = append(resp.PolicyResponse.Rules, *ruleResp)
		}
	}

	return resp
}
