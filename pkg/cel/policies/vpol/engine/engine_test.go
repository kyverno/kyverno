package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

func compileTestPolicy(t *testing.T, name string, expression string) Policy {
	t.Helper()
	pol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: expression},
			},
		},
	}
	c := vpolcompiler.NewCompiler()
	compiled, errs := c.Compile(pol, nil)
	require.Empty(t, errs, "failed to compile test policy %s", name)
	return Policy{
		Actions:        sets.New[admissionregistrationv1.ValidationAction](admissionregistrationv1.Deny),
		Policy:         pol,
		CompiledPolicy: compiled,
	}
}

func testResource() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "default",
			},
		},
	}
}

func TestHandle_ConcurrentEvaluation(t *testing.T) {
	const numPolicies = 20
	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = compileTestPolicy(t, "policy-pass-"+string(rune('a'+i)), "true")
	}

	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)

	request := engine.RequestFromJSON(nil, testResource())
	response, err := eng.Handle(context.Background(), request, nil)

	assert.NoError(t, err)
	assert.Len(t, response.Policies, numPolicies)
	for _, pol := range response.Policies {
		require.NotEmpty(t, pol.Rules)
		assert.Equal(t, engineapi.RuleStatusPass, pol.Rules[0].Status())
	}
}

func TestHandle_ConcurrentEvaluationMixedResults(t *testing.T) {
	policies := []Policy{
		compileTestPolicy(t, "pass-1", "true"),
		compileTestPolicy(t, "fail-1", "false"),
		compileTestPolicy(t, "pass-2", "true"),
		compileTestPolicy(t, "fail-2", "false"),
		compileTestPolicy(t, "pass-3", "true"),
	}

	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)

	request := engine.RequestFromJSON(nil, testResource())
	response, err := eng.Handle(context.Background(), request, nil)

	assert.NoError(t, err)
	assert.Len(t, response.Policies, 5)

	// Count pass/fail — order is preserved since we use index-based results
	passCount := 0
	failCount := 0
	for _, pol := range response.Policies {
		require.NotEmpty(t, pol.Rules)
		if pol.Rules[0].Status() == engineapi.RuleStatusPass {
			passCount++
		} else if pol.Rules[0].Status() == engineapi.RuleStatusFail {
			failCount++
		}
	}
	assert.Equal(t, 3, passCount)
	assert.Equal(t, 2, failCount)
}

func TestHandle_EmptyPolicies(t *testing.T) {
	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return nil, nil
	})
	eng := NewEngine(provider, nil, nil)

	request := engine.RequestFromJSON(nil, testResource())
	response, err := eng.Handle(context.Background(), request, nil)

	assert.NoError(t, err)
	assert.Empty(t, response.Policies)
}

func TestHandle_PredicateFiltering(t *testing.T) {
	policies := []Policy{
		compileTestPolicy(t, "include-me", "true"),
		compileTestPolicy(t, "exclude-me", "true"),
		compileTestPolicy(t, "include-also", "true"),
	}

	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)

	// Predicate is only applied in admission path, not JSON path
	// So for JSON path all policies are evaluated regardless
	predicate := func(p policiesv1beta1.ValidatingPolicyLike) bool {
		return p.GetName() != "exclude-me"
	}

	request := engine.RequestFromJSON(nil, testResource())
	response, err := eng.Handle(context.Background(), request, predicate)

	assert.NoError(t, err)
	// JSON path doesn't apply predicate — all 3 evaluated
	assert.Len(t, response.Policies, 3)
}

func TestHandle_LargePolicySet(t *testing.T) {
	// Stress test with many policies to exercise concurrency
	const numPolicies = 100
	policies := make([]Policy, numPolicies)
	for i := range policies {
		expr := "true"
		if i%3 == 0 {
			expr = "false"
		}
		policies[i] = compileTestPolicy(t, "policy-"+string(rune('0'+i/100%10))+string(rune('0'+i/10%10))+string(rune('0'+i%10)), expr)
	}

	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)

	request := engine.RequestFromJSON(nil, testResource())
	response, err := eng.Handle(context.Background(), request, nil)

	assert.NoError(t, err)
	assert.Len(t, response.Policies, numPolicies, "all policies must be evaluated")

	passCount := 0
	failCount := 0
	for _, pol := range response.Policies {
		require.NotEmpty(t, pol.Rules)
		if pol.Rules[0].Status() == engineapi.RuleStatusPass {
			passCount++
		} else {
			failCount++
		}
	}
	// Every 3rd policy (i%3==0) fails: indices 0,3,6,...,99 = 34 policies
	assert.Equal(t, 34, failCount)
	assert.Equal(t, 66, passCount)
}
