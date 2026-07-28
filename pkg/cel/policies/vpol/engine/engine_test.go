package engine

import (
	"context"
	"fmt"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

// buildJSONPolicy creates a ValidatingPolicy in JSON evaluation mode with the
// given validation expressions for use in unit tests.
func buildJSONPolicy(name string, validations []admissionregistrationv1.Validation) *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.EvaluationConfiguration{
				Mode: policieskyvernoio.EvaluationModeJSON,
			},
			Validations: validations,
		},
	}
}

func TestHandle_ValidationIndexInProperties(t *testing.T) {
	// Four expressions; only the third (index 2) fails.
	// cel.validationIndex in the response properties must be "2".
	policy := buildJSONPolicy("test-index", []admissionregistrationv1.Validation{
		{Expression: "object.name == 'allowed'", Message: "index 0: passes"},
		{Expression: "size(object.name) > 0", Message: "index 1: passes"},
		{Expression: "object.name == 'forbidden'", Message: "index 2: fails"},
		{Expression: "object.name != ''", Message: "index 3: would pass"},
	})

	provider, err := NewProvider(vpolcompiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)

	eng := NewEngine(provider, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "allowed"}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	rule := resp.Policies[0].Rules[0]
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Contains(t, rule.Message(), "index 2: fails")
	assert.Equal(t, "2", rule.Properties()["cel.validationIndex"],
		"cel.validationIndex must reflect the actual failing expression index, not the loop counter")
}

func TestHandle_ValidationIndexFirstExpression(t *testing.T) {
	// When the first expression fails, cel.validationIndex must be "0".
	policy := buildJSONPolicy("test-index-first", []admissionregistrationv1.Validation{
		{Expression: "object.name == 'wrong'", Message: "index 0: fails"},
		{Expression: "object.name != ''", Message: "index 1: would pass"},
	})

	provider, err := NewProvider(vpolcompiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)

	eng := NewEngine(provider, nil, nil)
	payload := &unstructured.Unstructured{Object: map[string]any{"name": "allowed"}}

	resp, err := eng.Handle(context.Background(), celengine.RequestFromJSON(nil, payload), nil)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)

	rule := resp.Policies[0].Rules[0]
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "0", rule.Properties()["cel.validationIndex"])
}

func TestWithValidationIndex(t *testing.T) {
	t.Run("nil props", func(t *testing.T) {
		out := withValidationIndex(nil, 3)
		assert.Equal(t, "3", out["cel.validationIndex"])
	})

	t.Run("existing props are preserved", func(t *testing.T) {
		props := map[string]string{"existing-key": "existing-value"}
		out := withValidationIndex(props, 1)
		assert.Equal(t, "1", out["cel.validationIndex"])
		assert.Equal(t, "existing-value", out["existing-key"])
	})

	t.Run("does not mutate original map", func(t *testing.T) {
		props := map[string]string{"key": "val"}
		_ = withValidationIndex(props, 5)
		_, exists := props["cel.validationIndex"]
		assert.False(t, exists, "original props map must not be mutated")
	})

	t.Run("existing cel.validationIndex is not overwritten", func(t *testing.T) {
		props := map[string]string{"cel.validationIndex": "user-defined"}
		out := withValidationIndex(props, 2)
		assert.Equal(t, "user-defined", out["cel.validationIndex"],
			"user-defined cel.validationIndex must not be clobbered by the engine")
	})
}

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
	request := celengine.RequestFromJSON(nil, testResource())

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
	request := celengine.RequestFromJSON(nil, testResource())

	response, err := eng.Handle(context.Background(), request, nil)
	assert.NoError(t, err)
	assert.Len(t, response.Policies, 5)

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
	request := celengine.RequestFromJSON(nil, testResource())

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

	predicate := func(p policiesv1beta1.ValidatingPolicyLike) bool {
		return p.GetName() != "exclude-me"
	}
	request := celengine.RequestFromJSON(nil, testResource())

	response, err := eng.Handle(context.Background(), request, predicate)
	assert.NoError(t, err)
	// JSON path doesn't apply predicate — all 3 evaluated
	assert.Len(t, response.Policies, 3)
}

func TestHandle_LargePolicySet(t *testing.T) {
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
	request := celengine.RequestFromJSON(nil, testResource())

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

// vpol writes into an index-based slice, so order should already be stable —
// this test proves it across repeated runs.
func TestHandle_DeterministicOrder(t *testing.T) {
	const numPolicies = 20
	const numRuns = 15

	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = compileTestPolicy(t, fmt.Sprintf("policy-%02d", i), "true")
	}
	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)
	request := celengine.RequestFromJSON(nil, testResource())

	var first []string
	for run := 0; run < numRuns; run++ {
		response, err := eng.Handle(context.Background(), request, nil)
		require.NoError(t, err)
		require.Len(t, response.Policies, numPolicies)

		names := make([]string, numPolicies)
		for i, pol := range response.Policies {
			names[i] = pol.Policy.GetName()
		}
		if run == 0 {
			first = names
		} else {
			assert.Equal(t, first, names, "result order changed on run %d", run)
		}
	}
}

func TestHandle_ConcurrentEvaluation_AdmissionPath(t *testing.T) {
	const numPolicies = 10
	policies := make([]Policy, numPolicies)
	for i := range policies {
		policies[i] = compileTestPolicy(t, fmt.Sprintf("admission-pass-%02d", i), "true")
	}
	provider := ProviderFunc(func(ctx context.Context) ([]Policy, error) {
		return policies, nil
	})
	eng := NewEngine(provider, nil, nil)

	podJSON := `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"test-pod","namespace":"default"}}`
	request := celengine.EngineRequest{
		Request: admissionv1.AdmissionRequest{
			Operation:       admissionv1.Create,
			Kind:            metav1.GroupVersionKind{Version: "v1", Kind: "Pod"},
			Resource:        metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
			RequestResource: &metav1.GroupVersionResource{Version: "v1", Resource: "pods"},
			Object:          runtime.RawExtension{Raw: []byte(podJSON)},
		},
	}

	response, err := eng.Handle(context.Background(), request, nil)
	require.NoError(t, err)
	assert.Len(t, response.Policies, numPolicies)
}
