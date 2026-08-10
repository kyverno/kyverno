package engine

import (
	"context"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
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

	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
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

// jobSetWithImage builds a minimal JobSet-shaped object (same shape as the
// repro used against issue #16477) with the given container image.
func jobSetWithImage(image string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "jobset.x-k8s.io/v1alpha2",
		"kind":       "JobSet",
		"metadata":   map[string]any{"name": "latest-tag-jobset", "namespace": "default"},
		"spec": map[string]any{
			"replicatedJobs": []any{
				map[string]any{
					"name": "workers",
					"template": map[string]any{
						"spec": map[string]any{
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{"name": "worker", "image": image},
									},
								},
							},
						},
					},
				},
			},
		},
	}}
}

// buildDisallowLatestTagPolicy builds a Pod-targeted ValidatingPolicy, left
// completely unmodified, with autogen configured for a custom CRD via
// ExtractionReplacementsRef (see pkg/cel/policies/vpol/autogen).
func buildDisallowLatestTagPolicy() *policiesv1beta1.ValidatingPolicy {
	return &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "disallow-latest-tag"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{
								admissionregistrationv1.Create,
								admissionregistrationv1.Update,
							},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			AutogenConfiguration: &policiesv1beta1.ValidatingPolicyAutogenConfiguration{
				PodControllers: &policiesv1beta1.PodControllersGenerationConfiguration{
					Controllers: []string{"jobsets.v1alpha2.jobset.x-k8s.io"},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{Expression: "object.spec.containers.all(c, !c.image.endsWith(':latest'))"},
			},
		},
	}
}

func TestHandle_ExtractionMode_JobSet(t *testing.T) {
	policy := buildDisallowLatestTagPolicy()
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, nil)
	require.NoError(t, err)
	noopNsResolver := func(string) *corev1.Namespace { return nil }
	eng := NewEngine(provider, noopNsResolver, matching.NewMatcher())

	assertSingleRuleResult := func(t *testing.T, image string) *engineapi.RuleResponse {
		t.Helper()
		req := celengine.Request(
			nil,
			schema.GroupVersionKind{Group: "jobset.x-k8s.io", Version: "v1alpha2", Kind: "JobSet"},
			schema.GroupVersionResource{Group: "jobset.x-k8s.io", Version: "v1alpha2", Resource: "jobsets"},
			"",
			"latest-tag-jobset",
			"default",
			admissionv1.Create,
			authenticationv1.UserInfo{},
			jobSetWithImage(image),
			nil,
			false,
			nil,
		)
		resp, err := eng.Handle(context.Background(), req, nil)
		require.NoError(t, err)

		var fired []celengine.ValidatingPolicyResponse
		for _, p := range resp.Policies {
			if len(p.Rules) > 0 {
				fired = append(fired, p)
			}
		}
		require.Len(t, fired, 1, "exactly one policy entry (the extraction-mode JobSet target) should have fired, not the base Pod-targeted one")
		require.Len(t, fired[0].Rules, 1)
		return &fired[0].Rules[0]
	}

	t.Run("bad image is denied, with the failing template's path in the message", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:latest")
		assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
		assert.Contains(t, rule.Message(), "spec.replicatedJobs[0].template.spec.template")
	})

	t.Run("compliant image is allowed", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:1.0")
		assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
	})
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
