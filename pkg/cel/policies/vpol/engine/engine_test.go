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

// buildDisallowLatestTagPolicyUsingRequest is identical to
// buildDisallowLatestTagPolicy except its validation reads request.object
// instead of the top-level object variable, to verify extraction mode
// synthesizes a matching AdmissionRequest, not just matching admission
// attributes.
func buildDisallowLatestTagPolicyUsingRequest() *policiesv1beta1.ValidatingPolicy {
	policy := buildDisallowLatestTagPolicy()
	policy.Spec.Validations = []admissionregistrationv1.Validation{
		{Expression: "request.kind.kind == 'Pod' && request.object.spec.containers.all(c, !c.image.endsWith(':latest'))"},
	}
	return policy
}

func TestHandle_ExtractionMode_RequestObjectMatchesSynthesizedPod(t *testing.T) {
	policy := buildDisallowLatestTagPolicyUsingRequest()
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
		require.Len(t, fired, 1)
		require.Len(t, fired[0].Rules, 1)
		return &fired[0].Rules[0]
	}

	t.Run("request.object sees the synthesized Pod, not the JobSet - bad image is denied", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:latest")
		require.NotEqual(t, engineapi.RuleStatusError, rule.Status(), "message: %s", rule.Message())
		assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	})

	t.Run("request.object sees the synthesized Pod, not the JobSet - compliant image is allowed", func(t *testing.T) {
		rule := assertSingleRuleResult(t, "bash:1.0")
		require.NotEqual(t, engineapi.RuleStatusError, rule.Status(), "message: %s", rule.Message())
		assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
	})
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

// buildException creates a PolicyException targeting the named ValidatingPolicy, matching every
// resource, with the given compensating controls.
func buildException(namespace, name, policyName string, validations ...admissionregistrationv1.Validation) *policiesv1beta1.PolicyException {
	return &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			PolicyRefs:      []policiesv1beta1.PolicyRef{{Name: policyName, Kind: "ValidatingPolicy"}},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "always", Expression: "true"}},
			Validations:     validations,
		},
	}
}

func handle(t *testing.T, policy *policiesv1beta1.ValidatingPolicy, payload map[string]any, exceptions ...*policiesv1beta1.PolicyException) engineapi.RuleResponse {
	t.Helper()
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.ValidatingPolicyLike{policy}, exceptions)
	require.NoError(t, err)

	resp, err := NewEngine(provider, nil, nil).Handle(
		context.Background(),
		celengine.RequestFromJSON(nil, &unstructured.Unstructured{Object: payload}),
		nil,
	)
	require.NoError(t, err)
	require.Len(t, resp.Policies, 1)
	require.Len(t, resp.Policies[0].Rules, 1)
	return resp.Policies[0].Rules[0]
}

// denyingPolicy fails for every resource, so the only thing that can produce a pass or skip is an
// exception being granted.
func denyingPolicy() *policiesv1beta1.ValidatingPolicy {
	return buildJSONPolicy("compensating-controls", []admissionregistrationv1.Validation{
		{Expression: "false", Message: "base policy denied"},
	})
}

// admittingPolicy passes for every resource: nothing here needs an exception at all.
func admittingPolicy() *policiesv1beta1.ValidatingPolicy {
	return buildJSONPolicy("compensating-controls", []admissionregistrationv1.Validation{
		{Expression: "true", Message: "base policy would pass"},
	})
}

func ticketRequired() admissionregistrationv1.Validation {
	return admissionregistrationv1.Validation{Expression: "has(object.ticket)", Message: "ticket required"}
}

func TestHandle_CompensatingControlsSatisfied(t *testing.T) {
	// The control holds, so the exception is granted and the policy is bypassed.
	rule := handle(t, denyingPolicy(), map[string]any{"ticket": "SEC-1"},
		buildException("default", "polex", "compensating-controls", ticketRequired()),
	)
	assert.Equal(t, engineapi.RuleStatusSkip, rule.Status())
	assert.Contains(t, rule.Message(), "due to policy exception")
	assert.True(t, rule.IsException())
}

func TestHandle_CompensatingControlsRefusedAndPolicyFails(t *testing.T) {
	// The control fails so the exception grants nothing, and the policy denies the resource on
	// its own. The message must be the control's: it is the only place the submitter learns an
	// exception existed and what it would have taken to qualify.
	rule := handle(t, denyingPolicy(), map[string]any{},
		buildException("default", "polex", "compensating-controls", ticketRequired()),
	)
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "ticket required", rule.Message())
	assert.NotContains(t, rule.Message(), "base policy denied")
	// the refused exception is still recorded, so reports name it
	require.Len(t, rule.Exceptions(), 1)
	assert.Equal(t, "polex", rule.Exceptions()[0].GetName())
}

func TestHandle_CompensatingControlsRefusedButPolicyPasses(t *testing.T) {
	// A resource that satisfies the policy needs no exception, so a failing control must not be
	// able to deny it. Compensating controls gate the bypass, never the resource.
	rule := handle(t, admittingPolicy(), map[string]any{},
		buildException("default", "polex", "compensating-controls", ticketRequired()),
	)
	assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
	assert.Empty(t, rule.Exceptions())
}

func TestHandle_CompensatingControlsRefusedButAnotherExceptionGrants(t *testing.T) {
	// Exceptions compose as alternatives. One that demands a ticket and does not get it grants
	// nothing, but it must not veto an unconditional exception that already covers the resource.
	rule := handle(t, denyingPolicy(), map[string]any{},
		buildException("default", "b-open", "compensating-controls"),
		buildException("default", "a-strict", "compensating-controls", ticketRequired()),
	)
	assert.Equal(t, engineapi.RuleStatusSkip, rule.Status())
	assert.True(t, rule.IsException())
	require.Len(t, rule.Exceptions(), 1)
	assert.Equal(t, "b-open", rule.Exceptions()[0].GetName())
}

func TestHandle_CompensatingControlsMessageExpression(t *testing.T) {
	rule := handle(t, denyingPolicy(), map[string]any{"app": "legacy-app-1"},
		buildException("default", "polex", "compensating-controls",
			admissionregistrationv1.Validation{
				Expression:        "has(object.ticket)",
				Message:           "static message",
				MessageExpression: "'no ticket on ' + object.app",
			}),
	)
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "no ticket on legacy-app-1", rule.Message())
}

func TestHandle_CompensatingControlsFirstRefusalWins(t *testing.T) {
	// Two matching exceptions both refuse. Exceptions are compiled in sorted order, so the
	// reported message is the same on every request rather than following informer order.
	rule := handle(t, denyingPolicy(), map[string]any{},
		buildException("default", "b-second", "compensating-controls",
			admissionregistrationv1.Validation{Expression: "has(object.b)", Message: "second refusal"}),
		buildException("default", "a-first", "compensating-controls",
			admissionregistrationv1.Validation{Expression: "has(object.a)", Message: "first refusal"}),
	)
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "first refusal", rule.Message())
	require.Len(t, rule.Exceptions(), 1)
	assert.Equal(t, "a-first", rule.Exceptions()[0].GetName())
}

func TestHandle_CompensatingControlsNotEvaluatedWhenExceptionDoesNotMatch(t *testing.T) {
	// The controls gate the exception, not the resource: if the exception's matchConditions do not
	// match, its controls are irrelevant and the base policy applies as usual.
	polex := buildException("default", "polex", "compensating-controls", ticketRequired())
	polex.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
		{Name: "never", Expression: "false"},
	}
	rule := handle(t, denyingPolicy(), map[string]any{}, polex)
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "base policy denied", rule.Message())
	assert.False(t, rule.IsException())
}

func TestHandle_CompensatingControlsNotReportedWhenPolicyExcludesResource(t *testing.T) {
	// The policy's own matchConditions put this resource out of scope, so the policy never
	// applies. A refused exception must not resurrect it as a denial.
	policy := denyingPolicy()
	policy.Spec.MatchConditions = []admissionregistrationv1.MatchCondition{
		{Name: "in-scope", Expression: "has(object.inScope)"},
	}
	rule := handle(t, policy, map[string]any{},
		buildException("default", "polex", "compensating-controls", ticketRequired()),
	)
	assert.Equal(t, engineapi.RuleStatusSkip, rule.Status())
	assert.Empty(t, rule.Exceptions())
}

func TestHandle_CompensatingControlsEvaluationErrorAndPolicyFails(t *testing.T) {
	// A control that cannot be evaluated leaves us unable to tell whether the bypass was earned,
	// so it surfaces as an error rather than silently granting or denying the exception.
	rule := handle(t, denyingPolicy(), map[string]any{},
		buildException("default", "polex", "compensating-controls",
			// compiles to bool, but the key is absent at evaluation time
			admissionregistrationv1.Validation{Expression: "object.missing == 'x'", Message: "unreachable"}),
	)
	assert.Equal(t, engineapi.RuleStatusError, rule.Status())
	assert.Contains(t, rule.Message(), "failed to evaluate compensating controls")
	require.Len(t, rule.Exceptions(), 1)
}

func TestHandle_CompensatingControlsEvaluationErrorButPolicyPasses(t *testing.T) {
	// Even an unevaluatable control is moot once the resource satisfies the policy on its own.
	rule := handle(t, admittingPolicy(), map[string]any{},
		buildException("default", "polex", "compensating-controls",
			admissionregistrationv1.Validation{Expression: "object.missing == 'x'", Message: "unreachable"}),
	)
	assert.Equal(t, engineapi.RuleStatusPass, rule.Status())
}

func TestHandle_CompensatingControlsWithAllowedValues(t *testing.T) {
	// A refused exception contributes nothing at all, not even its allowedValues, and the
	// refusal is carried through a real policy evaluation rather than short-circuiting it.
	policy := buildJSONPolicy("compensating-controls", []admissionregistrationv1.Validation{
		{Expression: "object.name in exceptions.allowedValues", Message: "name not allowed"},
	})
	rule := handle(t, policy, map[string]any{"name": "other"},
		func() *policiesv1beta1.PolicyException {
			polex := buildException("default", "polex", "compensating-controls", ticketRequired())
			polex.Spec.AllowedValues = []string{"blessed"}
			return polex
		}(),
	)
	assert.Equal(t, engineapi.RuleStatusFail, rule.Status())
	assert.Equal(t, "ticket required", rule.Message())
}

func TestNewProvider_CompensatingControlCompileErrorNamesTheException(t *testing.T) {
	// A bad expression in an exception must be reported against the exception, not the policy's
	// own spec, otherwise the operator has no way to tell which resource is at fault.
	policy := buildJSONPolicy("compensating-controls", []admissionregistrationv1.Validation{
		{Expression: "true"},
	})
	polex := buildException("prod", "needs-ticket", "compensating-controls",
		admissionregistrationv1.Validation{Expression: "'not a bool'"})

	_, err := NewProvider(compiler.NewCompiler(),
		[]policiesv1beta1.ValidatingPolicyLike{policy},
		[]*policiesv1beta1.PolicyException{polex})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exception[prod/needs-ticket].spec.validations[0].expression")
}

func TestNewProvider_CompensatingControlCannotReferencePolicyScopedIdentifiers(t *testing.T) {
	// `variables` and `exceptions` belong to the policy and hold nothing at the point an
	// exception is evaluated. Leaving them undeclared turns what would be an unresolvable
	// attribute at admission time into a compile error the author sees immediately.
	for _, expression := range []string{"variables.env == 'prod'", "object.image in exceptions.allowedImages"} {
		t.Run(expression, func(t *testing.T) {
			policy := buildJSONPolicy("compensating-controls", []admissionregistrationv1.Validation{
				{Expression: "true"},
			})
			policy.Spec.Variables = []admissionregistrationv1.Variable{{Name: "env", Expression: "'prod'"}}
			polex := buildException("prod", "needs-ticket", "compensating-controls",
				admissionregistrationv1.Validation{Expression: expression})

			_, err := NewProvider(compiler.NewCompiler(),
				[]policiesv1beta1.ValidatingPolicyLike{policy},
				[]*policiesv1beta1.PolicyException{polex})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "exception[prod/needs-ticket].spec.validations[0].expression")
		})
	}
}
