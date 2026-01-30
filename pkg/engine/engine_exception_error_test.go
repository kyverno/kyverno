package engine

import (
	"context"
	"fmt"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/factories"
	"github.com/kyverno/kyverno/pkg/imageverifycache"
	"github.com/stretchr/testify/assert"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestInvokeRuleHandler_ExceptionLookupError_ReturnsRuleError(t *testing.T) {
	mockSelector := new(MockExceptionSelector)
	mockSelector.On("Find", "test-policy", "validate-tag").
		Return(([]*kyvernov2.PolicyException)(nil), fmt.Errorf("exception lister unavailable"))

	e := NewEngine(cfg, jp, nil, nil, imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil), mockSelector, nil)

	policy := &kyverno.ClusterPolicy{}
	policy.SetName("test-policy")
	policy.Spec = kyverno.Spec{
		Rules: []kyverno.Rule{{
			Name: "validate-tag",
			MatchResources: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
			Validation: &kyverno.Validation{
				Message: "test validation",
				RawPattern: &apiextv1.JSON{
					Raw: []byte(`{"metadata":{"labels":{"app":"*"}}}`),
				},
			},
		}},
	}

	var res unstructured.Unstructured
	res.SetAPIVersion("v1")
	res.SetKind("Pod")
	res.SetName("test-pod")
	res.SetNamespace("default")
	res.Object["metadata"] = map[string]interface{}{
		"name":      "test-pod",
		"namespace": "default",
		"labels":    map[string]interface{}{"app": "web"},
	}

	pCtx, err := NewPolicyContext(jp, res, kyverno.Create, nil, cfg)
	assert.NoError(t, err)
	pCtx = pCtx.WithPolicy(policy)

	resp := e.Validate(context.TODO(), pCtx)

	// Must have at least one rule response
	assert.NotEmpty(t, resp.PolicyResponse.Rules, "expected rule responses when exception lookup fails")

	// The rule response must be RuleStatusError, not silently skipped
	foundError := false
	for _, rule := range resp.PolicyResponse.Rules {
		if rule.Status() == engineapi.RuleStatusError {
			foundError = true
			assert.Contains(t, rule.Message(), "failed to get exceptions")
		}
	}
	assert.True(t, foundError, "expected RuleStatusError when exception lookup fails, got: %v", resp.PolicyResponse.Rules)
	mockSelector.AssertExpectations(t)
}

func TestFilterRule_ExceptionLookupError_ReturnsRuleError(t *testing.T) {
	mockSelector := new(MockExceptionSelector)
	mockSelector.On("Find", "test-policy", "generate-config").
		Return(([]*kyvernov2.PolicyException)(nil), fmt.Errorf("exception lister unavailable"))

	e := NewEngine(cfg, jp, nil, nil, imageverifycache.DisabledImageVerifyCache(),
		factories.DefaultContextLoaderFactory(nil), mockSelector, nil)

	policy := &kyverno.ClusterPolicy{}
	policy.SetName("test-policy")
	policy.Spec = kyverno.Spec{
		Rules: []kyverno.Rule{{
			Name: "generate-config",
			MatchResources: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"ConfigMap"},
				},
			},
			Generation: &kyverno.Generation{
				Synchronize: true,
			},
		}},
	}

	var res unstructured.Unstructured
	res.SetAPIVersion("v1")
	res.SetKind("ConfigMap")
	res.SetName("test-cm")
	res.SetNamespace("default")

	pCtx, err := NewPolicyContext(jp, res, kyverno.Create, nil, cfg)
	assert.NoError(t, err)
	pCtx = pCtx.WithPolicy(policy)

	resp := e.ApplyBackgroundChecks(context.TODO(), pCtx)

	// Must have at least one rule response
	assert.NotEmpty(t, resp.PolicyResponse.Rules, "expected rule responses when exception lookup fails")

	// The rule response must be RuleStatusError, not silently skipped
	foundError := false
	for _, rule := range resp.PolicyResponse.Rules {
		if rule.Status() == engineapi.RuleStatusError {
			foundError = true
			assert.Contains(t, rule.Message(), "failed to get exceptions")
		}
	}
	assert.True(t, foundError, "expected RuleStatusError when exception lookup fails, got: %v", resp.PolicyResponse.Rules)
	mockSelector.AssertExpectations(t)
}
