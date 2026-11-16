package admissionpolicy

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/cel-go/cel"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernocompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

func TestBuildMutatingAdmissionPolicyBeta(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
			Labels: map[string]string{
				"test-label": "test-value",
			},
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						ResourceNames: []string{"test-resource"},
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
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "test-condition",
					Expression: "true",
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: "Object{spec: Object{replicas: 3}}",
					},
				},
			},
			Variables: []admissionregistrationv1.Variable{
				{
					Name:       "test-var",
					Expression: "'test-value'",
				},
			},
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol",
		},
	}

	exceptions := []policiesv1beta1.PolicyException{
		{
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "exception-condition",
						Expression: "object.metadata.name == 'skip'",
					},
				},
			},
		},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, exceptions)

	// Verify owner reference
	assert.Len(t, mapol.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapol.OwnerReferences[0].Name)
	assert.Equal(t, mp.GetUID(), mapol.OwnerReferences[0].UID)

	// Verify match constraints
	assert.NotNil(t, mapol.Spec.MatchConstraints)
	assert.Len(t, mapol.Spec.MatchConstraints.ResourceRules, 1)
	assert.Equal(t, "test-resource", mapol.Spec.MatchConstraints.ResourceRules[0].ResourceNames[0])

	// Verify match conditions (original + negated exceptions)
	assert.Len(t, mapol.Spec.MatchConditions, 2)
	// First condition should be the exception (negated)
	assert.Equal(t, "exception-condition", mapol.Spec.MatchConditions[0].Name)
	assert.Equal(t, "!(object.metadata.name == 'skip')", mapol.Spec.MatchConditions[0].Expression)
	// Second condition should be the original policy condition
	assert.Equal(t, "test-condition", mapol.Spec.MatchConditions[1].Name)
	assert.Equal(t, "true", mapol.Spec.MatchConditions[1].Expression)

	// Verify mutations
	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)

	// Verify variables
	assert.Len(t, mapol.Spec.Variables, 1)
	assert.Equal(t, "test-var", mapol.Spec.Variables[0].Name)

	// Verify labels
	assert.NotNil(t, mapol.Labels)
	assert.Contains(t, mapol.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyV1(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "test-condition", Expression: "true"}},
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
				ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
					Expression: "Object{spec: Object{replicas: 3}}",
				},
			}},
			Variables: []admissionregistrationv1.Variable{{Name: "test-var", Expression: "'test-value'"}},
		},
	}

	mapol := &admissionregistrationv1.MutatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol"}}

	BuildMutatingAdmissionPolicyV1(mapol, mp, nil)

	assert.Len(t, mapol.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapol.OwnerReferences[0].Name)
	assert.NotNil(t, mapol.Spec.MatchConstraints)
	assert.Len(t, mapol.Spec.MatchConstraints.ResourceRules, 1)
	assert.Len(t, mapol.Spec.MatchConditions, 1)
	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol.Spec.Mutations[0].ApplyConfiguration)
	assert.Equal(t, "Object{spec: Object{replicas: 3}}", mapol.Spec.Mutations[0].ApplyConfiguration.Expression)
	assert.Equal(t, admissionregistrationv1.ReinvocationPolicyType(mp.Spec.GetReinvocationPolicy()), mapol.Spec.ReinvocationPolicy)
	assert.Len(t, mapol.Spec.Variables, 1)
	assert.Contains(t, mapol.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBindingV1(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-mpol", UID: "test-uid"}}
	mapbinding := &admissionregistrationv1.MutatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol-binding"}}

	BuildMutatingAdmissionPolicyBindingV1(mapbinding, mp)

	assert.Len(t, mapbinding.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapbinding.OwnerReferences[0].Name)
	assert.Equal(t, "mpol-test-mpol", mapbinding.Spec.PolicyName)
	assert.Contains(t, mapbinding.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBindingBeta(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
		},
	}

	mapbinding := &admissionregistrationv1beta1.MutatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol-binding",
		},
	}

	BuildMutatingAdmissionPolicyBindingBeta(mapbinding, mp)

	// Verify owner reference
	assert.Len(t, mapbinding.OwnerReferences, 1)
	assert.Equal(t, mp.GetName(), mapbinding.OwnerReferences[0].Name)
	assert.Equal(t, mp.GetUID(), mapbinding.OwnerReferences[0].UID)

	// Verify policy name
	assert.Equal(t, "mpol-test-mpol", mapbinding.Spec.PolicyName)

	// Verify labels
	assert.NotNil(t, mapbinding.Labels)
	assert.Contains(t, mapbinding.Labels, "app.kubernetes.io/managed-by")
}

func TestBuildMutatingAdmissionPolicyBeta_WithFailurePolicy(t *testing.T) {
	failurePolicy := admissionregistrationv1.Fail
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			FailurePolicy: (*admissionregistrationv1.FailurePolicyType)(&failurePolicy),
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mpol-test-mpol",
		},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, nil)

	// Verify failure policy
	assert.NotNil(t, mapol.Spec.FailurePolicy)
	assert.Equal(t, admissionregistrationv1beta1.Fail, *mapol.Spec.FailurePolicy)
}

func TestBuildMutatingAdmissionPolicyBeta_MutationTypeConversion(t *testing.T) {
	// Test ApplyConfiguration mutation
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: "Object{metadata: Object{labels: {'app': 'test'}}}",
					},
				},
			},
		},
	}

	mapol := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
	}

	BuildMutatingAdmissionPolicyBeta(mapol, mp, nil)

	assert.Len(t, mapol.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeApplyConfiguration, mapol.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol.Spec.Mutations[0].ApplyConfiguration)
	assert.Equal(t, "Object{metadata: Object{labels: {'app': 'test'}}}", mapol.Spec.Mutations[0].ApplyConfiguration.Expression)

	// Test JSONPatch mutation
	mp2 := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
					JSONPatch: &admissionregistrationv1alpha1.JSONPatch{
						Expression: "[{'op': 'add', 'path': '/metadata/labels/app', 'value': 'test'}]",
					},
				},
			},
		},
	}

	mapol2 := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test2"},
	}

	BuildMutatingAdmissionPolicyBeta(mapol2, mp2, nil)

	assert.Len(t, mapol2.Spec.Mutations, 1)
	assert.Equal(t, admissionregistrationv1beta1.PatchTypeJSONPatch, mapol2.Spec.Mutations[0].PatchType)
	assert.NotNil(t, mapol2.Spec.Mutations[0].JSONPatch)
	assert.Equal(t, "[{'op': 'add', 'path': '/metadata/labels/app', 'value': 'test'}]", mapol2.Spec.Mutations[0].JSONPatch.Expression)
}

func TestCELStringListLiteral_ValuesStayLiterals(t *testing.T) {
	// Each value is what a PolicyException author puts in spec.images or
	// spec.allowedValues. Every one of them must survive as a plain string.
	tests := []struct {
		name  string
		value string
	}{
		{"plain image reference", "ghcr.io/kyverno/kyverno:v1.19.0"},
		{"quote, brackets and an object path", "x'] + [object.spec.containers[0].image] + ['y"},
		{"quote and an object path", "a' + object.metadata.name + '"},
		{"quote, brackets and a division", "x'] + [string(1/0)] + ['y"},
		{"unbalanced single quote", "a'"},
		{"double quote", `weird"quote`},
		{"backslash", `back\slash`},
		{"newline", "new\nline"},
	}

	env, err := cel.NewEnv(cel.Variable("object", cel.DynType))
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expression, err := celStringListLiteral([]string{tt.value})
			require.NoError(t, err)

			// The rendered expression must be valid CEL, so the generated policy is
			// always writable. Interpolating the value produced a syntax error for the
			// quote, backslash and newline cases, which left a stale policy enforcing.
			ast, issues := env.Compile(expression)
			require.NoError(t, issues.Err())

			// It must evaluate to exactly the author's string, with nothing resolved
			// from the object under review.
			program, err := env.Program(ast)
			require.NoError(t, err)
			out, _, err := program.Eval(map[string]any{"object": map[string]any{}})
			require.NoError(t, err)

			native, err := out.ConvertToNative(reflect.TypeOf([]string{}))
			require.NoError(t, err)
			assert.Equal(t, []string{tt.value}, native)
		})
	}
}

func TestCELStringListLiteral_Shapes(t *testing.T) {
	expression, err := celStringListLiteral([]string{"a", "b"})
	assert.NoError(t, err)
	assert.Equal(t, `["a","b"]`, expression)

	expression, err = celStringListLiteral(nil)
	assert.NoError(t, err)
	assert.Equal(t, "null", expression)
}

func TestNegateMatchConditions(t *testing.T) {
	tests := []struct {
		name               string
		exceptionNamespace string
		exceptionName      string
		conditions         []admissionregistrationv1.MatchCondition
		expectedNil        bool
		expectedName       string
		expectedPrefix     string
		expectedExpr       string
	}{
		{
			name:               "empty conditions returns nil",
			exceptionNamespace: "default",
			exceptionName:      "empty-polex",
			conditions:         nil,
			expectedNil:        true,
		},
		{
			name:               "single condition preserves condition name and negates expression",
			exceptionNamespace: "default",
			exceptionName:      "single-polex",
			conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "check-name",
					Expression: "object.metadata.name == 'skip'",
				},
			},
			expectedNil:  false,
			expectedName: "check-name",
			expectedExpr: "!(object.metadata.name == 'skip')",
		},
		{
			name:               "multiple conditions combined into conjunction negation",
			exceptionNamespace: "default",
			exceptionName:      "multi-polex",
			conditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "admission-only",
					Expression: "has(request.userInfo.username)",
				},
				{
					Name:       "match-legacy",
					Expression: "request.resource.resource == 'accessentries'",
				},
			},
			expectedNil:    false,
			expectedPrefix: "polex-default-multi-polex-",
			expectedExpr:   "!((has(request.userInfo.username)) && (request.resource.resource == 'accessentries'))",
		},
		{
			name:               "three conditions properly grouped with parentheses",
			exceptionNamespace: "default",
			exceptionName:      "three-polex",
			conditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "object.metadata.namespace == 'kube-system'"},
				{Name: "c2", Expression: "object.metadata.name == 'runner'"},
				{Name: "c3", Expression: "request.operation == 'CREATE'"},
			},
			expectedNil:    false,
			expectedPrefix: "polex-default-three-polex-",
			expectedExpr:   "!((object.metadata.namespace == 'kube-system') && (object.metadata.name == 'runner') && (request.operation == 'CREATE'))",
		},
		{
			name:               "long exception name truncated with deterministic sha256 suffix within 63 chars",
			exceptionNamespace: "prod",
			exceptionName:      "this-is-a-very-long-exception-name-that-exceeds-the-kubernetes-sixty-three-character-limit",
			conditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "true"},
				{Name: "c2", Expression: "false"},
			},
			expectedNil:  false,
			expectedExpr: "!((true) && (false))",
		},
		{
			name:               "name ending in dot or hyphen before suffix boundary is trimmed",
			exceptionNamespace: "",
			// polex- (6 chars) + 47 'a's (47 chars) + . (1 char) = 54 chars, ending with '.' before suffix
			exceptionName: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.abcdefghij",
			conditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "true"},
				{Name: "c2", Expression: "false"},
			},
			expectedNil:  false,
			expectedExpr: "!((true) && (false))",
		},
		{
			name:               "single condition with trailing line comment delimited safely",
			exceptionNamespace: "default",
			exceptionName:      "comment-single-polex",
			conditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "true // allow single"},
			},
			expectedNil:  false,
			expectedExpr: "!(true // allow single\n)",
		},
		{
			name:               "trailing line comments delimited safely by newlines",
			exceptionNamespace: "default",
			exceptionName:      "comment-polex",
			conditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "true // allow first"},
				{Name: "c2", Expression: "false // block second"},
			},
			expectedNil:    false,
			expectedPrefix: "polex-default-comment-polex-",
			expectedExpr:   "!((true // allow first\n) && (false // block second\n))",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := negateMatchConditions(tt.exceptionNamespace, tt.exceptionName, tt.conditions)
			if tt.expectedNil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				// verify determinism across reconciliations
				result2 := negateMatchConditions(tt.exceptionNamespace, tt.exceptionName, tt.conditions)
				require.NotNil(t, result2)
				assert.Equal(t, result.Name, result2.Name, "generated condition name must be deterministic across reconciliations")
				assert.LessOrEqual(t, len(result.Name), 63)
				assert.False(t, strings.HasSuffix(result.Name, "-"))
				assert.False(t, strings.HasSuffix(result.Name, "."))
				assert.NotContains(t, result.Name, ".-")
				if tt.expectedName != "" {
					assert.Equal(t, tt.expectedName, result.Name)
				}
				if tt.expectedPrefix != "" {
					assert.True(t, strings.HasPrefix(result.Name, tt.expectedPrefix))
				}
				assert.Equal(t, tt.expectedExpr, result.Expression)
			}
		})
	}
}

func TestNegateMatchConditions_UniqueSuffixForSharedPrefix(t *testing.T) {
	mcs := []admissionregistrationv1.MatchCondition{
		{Name: "c1", Expression: "true"},
		{Name: "c2", Expression: "false"},
	}
	sharedPrefix := "shared-very-long-prefix-that-exceeds-the-limit-and-needs-truncation-"
	r1 := negateMatchConditions("ns1", sharedPrefix+"variant-one", mcs)
	r2 := negateMatchConditions("ns1", sharedPrefix+"variant-two", mcs)
	r3 := negateMatchConditions("ns2", sharedPrefix+"variant-one", mcs)

	require.NotNil(t, r1)
	require.NotNil(t, r2)
	require.NotNil(t, r3)

	assert.NotEqual(t, r1.Name, r2.Name, "different names with same prefix must have unique condition names")
	assert.NotEqual(t, r1.Name, r3.Name, "same name in different namespaces must have unique condition names")
	assert.LessOrEqual(t, len(r1.Name), 63)
	assert.LessOrEqual(t, len(r2.Name), 63)
	assert.LessOrEqual(t, len(r3.Name), 63)
}

func TestNegateMatchConditions_ShortNameDifferentNamespaces(t *testing.T) {
	mcs := []admissionregistrationv1.MatchCondition{
		{Name: "c1", Expression: "true"},
		{Name: "c2", Expression: "false"},
	}
	r1 := negateMatchConditions("ns1", "exempt", mcs)
	r2 := negateMatchConditions("ns2", "exempt", mcs)
	r3 := negateMatchConditions("", "exempt", mcs)

	require.NotNil(t, r1)
	require.NotNil(t, r2)
	require.NotNil(t, r3)

	assert.True(t, strings.HasPrefix(r1.Name, "polex-ns1-exempt-"))
	assert.True(t, strings.HasPrefix(r2.Name, "polex-ns2-exempt-"))
	assert.True(t, strings.HasPrefix(r3.Name, "polex-exempt-"))
	assert.NotEqual(t, r1.Name, r2.Name)
	assert.NotEqual(t, r1.Name, r3.Name)
	assert.LessOrEqual(t, len(r1.Name), 63)
	assert.LessOrEqual(t, len(r2.Name), 63)
	assert.LessOrEqual(t, len(r3.Name), 63)
}

func TestNegateMatchConditions_AmbiguousNamespaceNameCollision(t *testing.T) {
	mcs := []admissionregistrationv1.MatchCondition{
		{Name: "c1", Expression: "true"},
		{Name: "c2", Expression: "false"},
	}
	r1 := negateMatchConditions("a-b", "c", mcs)
	r2 := negateMatchConditions("a", "b-c", mcs)

	require.NotNil(t, r1)
	require.NotNil(t, r2)

	assert.NotEqual(t, r1.Name, r2.Name, "ambiguous hyphenated namespace/name pairs must have unique condition names")
	assert.LessOrEqual(t, len(r1.Name), 63)
	assert.LessOrEqual(t, len(r2.Name), 63)
}

func TestBuildValidatingAdmissionPolicy_MultiConditionException(t *testing.T) {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "check-standard-labels",
			UID:  "vpol-uid-1",
		},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "has(object.metadata.labels) && 'app' in object.metadata.labels",
					Message:    "label 'app' is required",
				},
			},
		},
	}

	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "exempt-system-runner",
			Namespace: "kube-system",
		},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "in-kube-system",
					Expression: "object.metadata.namespace == 'kube-system'",
				},
				{
					Name:       "is-runner",
					Expression: "object.metadata.name == 'maintenance-runner'",
				},
			},
		},
	}

	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	err := BuildValidatingAdmissionPolicy(
		nil,
		vap,
		engineapi.NewValidatingPolicy(vpol),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(polex)},
	)
	require.NoError(t, err)

	require.Len(t, vap.Spec.MatchConditions, 1)
	assert.True(t, strings.HasPrefix(vap.Spec.MatchConditions[0].Name, "polex-kube-system-exempt-system-runner-"))
	assert.LessOrEqual(t, len(vap.Spec.MatchConditions[0].Name), 63)
	assert.Equal(t, "!((object.metadata.namespace == 'kube-system') && (object.metadata.name == 'maintenance-runner'))", vap.Spec.MatchConditions[0].Expression)
}

func TestBuildMutatingAdmissionPolicy_MultiConditionException(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-mpol",
			UID:  "test-uid",
		},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{""},
								APIVersions: []string{"v1"},
								Resources:   []string{"pods"},
							},
						},
					},
				},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "policy-check", Expression: "request.operation == 'CREATE'"},
			},
			Mutations: []admissionregistrationv1alpha1.Mutation{
				{
					PatchType: admissionregistrationv1alpha1.PatchTypeApplyConfiguration,
					ApplyConfiguration: &admissionregistrationv1alpha1.ApplyConfiguration{
						Expression: "Object{spec: Object{replicas: 1}}",
					},
				},
			},
		},
	}

	exceptions := []policiesv1beta1.PolicyException{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "exempt-legacy",
			},
			Spec: policiesv1beta1.PolicyExceptionSpec{
				MatchConditions: []admissionregistrationv1.MatchCondition{
					{
						Name:       "cond1",
						Expression: "request.userInfo.username != ''",
					},
					{
						Name:       "cond2",
						Expression: "request.resource.resource == 'pods'",
					},
				},
			},
		},
	}

	expectedExpr := "!((request.userInfo.username != '') && (request.resource.resource == 'pods'))"

	// V1 builder
	mapolV1 := &admissionregistrationv1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol"},
	}
	BuildMutatingAdmissionPolicyV1(mapolV1, mp, exceptions)
	require.Len(t, mapolV1.Spec.MatchConditions, 2)
	assert.True(t, strings.HasPrefix(mapolV1.Spec.MatchConditions[0].Name, "polex-exempt-legacy-"))
	assert.LessOrEqual(t, len(mapolV1.Spec.MatchConditions[0].Name), 63)
	assert.Equal(t, expectedExpr, mapolV1.Spec.MatchConditions[0].Expression)
	assert.Equal(t, "policy-check", mapolV1.Spec.MatchConditions[1].Name)

	// Alpha builder
	mapolAlpha := &admissionregistrationv1alpha1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol"},
	}
	BuildMutatingAdmissionPolicy(mapolAlpha, mp, exceptions)
	require.Len(t, mapolAlpha.Spec.MatchConditions, 2)
	assert.True(t, strings.HasPrefix(mapolAlpha.Spec.MatchConditions[0].Name, "polex-exempt-legacy-"))
	assert.LessOrEqual(t, len(mapolAlpha.Spec.MatchConditions[0].Name), 63)
	assert.Equal(t, expectedExpr, mapolAlpha.Spec.MatchConditions[0].Expression)
	assert.Equal(t, "policy-check", mapolAlpha.Spec.MatchConditions[1].Name)

	// Beta builder
	mapolBeta := &admissionregistrationv1beta1.MutatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mpol-test-mpol"},
	}
	BuildMutatingAdmissionPolicyBeta(mapolBeta, mp, exceptions)
	require.Len(t, mapolBeta.Spec.MatchConditions, 2)
	assert.True(t, strings.HasPrefix(mapolBeta.Spec.MatchConditions[0].Name, "polex-exempt-legacy-"))
	assert.LessOrEqual(t, len(mapolBeta.Spec.MatchConditions[0].Name), 63)
	assert.Equal(t, expectedExpr, mapolBeta.Spec.MatchConditions[0].Expression)
	assert.Equal(t, "policy-check", mapolBeta.Spec.MatchConditions[1].Name)
}

func TestBuildValidatingAdmissionPolicy_DuplicateConditionNameResolved(t *testing.T) {
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "vpol-test"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				// Policy intentionally already defines a condition with the same name as the exception
				{Name: "c1", Expression: "true"},
			},
		},
	}
	polex := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "test-polex"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "request.operation == 'CREATE'"},
			},
		},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	err := BuildValidatingAdmissionPolicy(
		nil,
		vap,
		engineapi.NewValidatingPolicy(vpol),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(polex)},
	)
	require.NoError(t, err)
	require.Len(t, vap.Spec.MatchConditions, 2)
	assert.Equal(t, "c1", vap.Spec.MatchConditions[0].Name)
	assert.Equal(t, "c1-1", vap.Spec.MatchConditions[1].Name)
}

func TestBuildMutatingAdmissionPolicy_DuplicateConditionNameResolved(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mpol-test"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			MatchConditions: []admissionregistrationv1.MatchCondition{
				// Policy intentionally defines a condition with the same name as the exception
				{Name: "c1", Expression: "true"},
			},
		},
	}
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "test-polex"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "c1", Expression: "request.operation == 'CREATE'"},
			},
		},
	}
	// Alpha builder
	mapolAlpha := &admissionregistrationv1alpha1.MutatingAdmissionPolicy{}
	BuildMutatingAdmissionPolicy(mapolAlpha, mpol, []policiesv1beta1.PolicyException{polex})
	require.Len(t, mapolAlpha.Spec.MatchConditions, 2)
	assert.Equal(t, "c1-1", mapolAlpha.Spec.MatchConditions[0].Name)
	assert.Equal(t, "c1", mapolAlpha.Spec.MatchConditions[1].Name)

	// V1 builder
	mapolV1 := &admissionregistrationv1.MutatingAdmissionPolicy{}
	BuildMutatingAdmissionPolicyV1(mapolV1, mpol, []policiesv1beta1.PolicyException{polex})
	require.Len(t, mapolV1.Spec.MatchConditions, 2)
	assert.Equal(t, "c1-1", mapolV1.Spec.MatchConditions[0].Name)
	assert.Equal(t, "c1", mapolV1.Spec.MatchConditions[1].Name)

	// Beta builder
	mapolBeta := &admissionregistrationv1beta1.MutatingAdmissionPolicy{}
	BuildMutatingAdmissionPolicyBeta(mapolBeta, mpol, []policiesv1beta1.PolicyException{polex})
	require.Len(t, mapolBeta.Spec.MatchConditions, 2)
	assert.Equal(t, "c1-1", mapolBeta.Spec.MatchConditions[0].Name)
	assert.Equal(t, "c1", mapolBeta.Spec.MatchConditions[1].Name)
}

func TestAllocateConditionName_PreservesDNSQualifiedPrefixOnCollision(t *testing.T) {
	// 60 'a' characters as DNS prefix + "/x"
	prefix := strings.Repeat("a", 60)
	condName := prefix + "/x"

	used := make(map[string]bool)
	first := allocateConditionName(used, condName)
	assert.Equal(t, condName, first)
	assert.Empty(t, validation.IsQualifiedName(first))

	// Second allocation for identical name must resolve collision by suffixing the local name,
	// preserving the DNS prefix without generating an invalid component like "/-1".
	second := allocateConditionName(used, condName)
	expectedSecond := prefix + "/x-1"
	assert.Equal(t, expectedSecond, second)
	assert.Empty(t, validation.IsQualifiedName(second))
}

func TestAllocateConditionName_LongLocalNameWithPrefixTruncatedSafely(t *testing.T) {
	prefix := "example.com"
	local := strings.Repeat("b", 63)
	condName := prefix + "/" + local

	used := make(map[string]bool)
	first := allocateConditionName(used, condName)
	assert.Equal(t, condName, first)
	assert.Empty(t, validation.IsQualifiedName(first))

	second := allocateConditionName(used, condName)
	assert.True(t, strings.HasPrefix(second, prefix+"/"))
	assert.Empty(t, validation.IsQualifiedName(second))
	assert.True(t, strings.HasSuffix(second, "-1"))
	localPart := strings.TrimPrefix(second, prefix+"/")
	assert.LessOrEqual(t, len(localPart), 63)
}

func TestNegateMatchConditions_CELSourceSizeLimitRegression(t *testing.T) {
	// Reproduces maintainer review finding:
	// Two conditions whose expression is '"' + strings.Repeat("a", 55000) + '"' != "":
	// both separately negated expressions pass compiler.CompileMatchConditionsWithKubernetesEnv,
	// but negateMatchConditions produces an expression exceeding the 100,000 code-point limit.
	expr := `"` + strings.Repeat("a", 55000) + `" != ""`
	conds := []admissionregistrationv1.MatchCondition{
		{Name: "c1", Expression: expr},
		{Name: "c2", Expression: expr},
	}

	// Verify both individual conditions pass CompileMatchConditionsWithKubernetesEnv
	errs1 := kyvernocompiler.CompileMatchConditionsWithKubernetesEnv([]admissionregistrationv1.MatchCondition{
		{Name: "c1", Expression: "!(" + expr + ")"},
	}, nil)
	assert.Empty(t, errs1, "individually negated condition should pass CEL compilation")

	errs2 := kyvernocompiler.CompileMatchConditionsWithKubernetesEnv([]admissionregistrationv1.MatchCondition{
		{Name: "c2", Expression: "!(" + expr + ")"},
	}, nil)
	assert.Empty(t, errs2, "individually negated condition should pass CEL compilation")

	// When combined by negateMatchConditions, code point count exceeds 100,000
	combined := negateMatchConditions("default", "oversized-polex", conds)
	require.NotNil(t, combined)
	codePoints := utf8.RuneCountInString(combined.Expression)
	assert.Greater(t, codePoints, MaxCELCodePoints)

	// CEL compiler rejects the combined expression due to code point size limit
	compilerErrs := kyvernocompiler.CompileMatchConditionsWithKubernetesEnv([]admissionregistrationv1.MatchCondition{*combined}, nil)
	require.NotEmpty(t, compilerErrs)
	assert.Contains(t, compilerErrs.ToAggregate().Error(), "expression code point size exceeds limit")

	// CanGenerateNativePolicy explicitly catches this unrepresentable exception
	polex := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "oversized-polex"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: conds,
		},
	}
	canGenerate, reason := CanGenerateNativePolicy([]policiesv1beta1.PolicyException{polex})
	assert.False(t, canGenerate)
	assert.Contains(t, reason, "exceeding the Kubernetes CEL code point limit (100000)")

	// BuildValidatingAdmissionPolicy defense-in-depth also rejects building the VAP
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "vpol-test"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
		},
	}
	err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), []engineapi.GenericException{
		engineapi.NewCELPolicyException(&polex),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds CEL code point limit (100000)")
}

func TestBuildValidatingAdmissionPolicy_DoesNotMutateSourcePolicy(t *testing.T) {
	t.Parallel()
	const original = "object.spec.containers.all(c, c.image in exceptions.allowedImages)"
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "check-images", UID: "test-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations:      []admissionregistrationv1.Validation{{Expression: original}},
		},
	}
	snapshot := vpol.DeepCopy()
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-check-images"}}

	require.NoError(t, BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), nil))

	require.Len(t, vap.Spec.Validations, 1)
	assert.Equal(t, "object.spec.containers.all(c, c.image in variables.allowedImages)", vap.Spec.Validations[0].Expression)
	assert.Equal(t, snapshot, vpol, "building a VAP must not mutate the (informer-cached) source policy")
}

func TestBuildValidatingAdmissionPolicy_DoesNotWriteIntoSourceSliceCapacity(t *testing.T) {
	t.Parallel()
	vars := make([]admissionregistrationv1.Variable, 1, 4) // spare capacity, like a decoded slice
	vars[0] = admissionregistrationv1.Variable{Name: "x", Expression: "1"}
	vpol := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p", UID: "u"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Variables:        vars,
			Validations:      []admissionregistrationv1.Validation{{Expression: "true"}},
		},
	}
	ex := engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{Images: []string{"nginx"}},
	})
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "vpol-p"}}

	require.NoError(t, BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(vpol), []engineapi.GenericException{ex}))

	require.Len(t, vap.Spec.Variables, 2)
	assert.Equal(t, admissionregistrationv1.Variable{}, vars[:cap(vars)][1],
		"append must not write into the source policy's backing array")
}
