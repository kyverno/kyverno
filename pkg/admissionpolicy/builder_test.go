package admissionpolicy

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/cel-go/cel"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestNegateExceptionMatchConditions(t *testing.T) {
	single := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "single"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "single-condition",
				Expression: "object.metadata.name == 'skip'",
			}},
		},
	}
	got := negateExceptionMatchConditions([]policiesv1beta1.PolicyException{single})
	require.Equal(t, []admissionregistrationv1.MatchCondition{{
		Name:       "single-condition",
		Expression: "!(object.metadata.name == 'skip')",
	}}, got)

	multi := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "multi"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "admission-only", Expression: "request.userInfo.username != ''"},
				{Name: "match-legacy-resources", Expression: `request.resource.resource == "accessentries"`},
			},
		},
	}
	got = negateExceptionMatchConditions([]policiesv1beta1.PolicyException{multi})
	require.Len(t, got, 1)
	assert.Regexp(t, `^exception-[0-9a-f]{12}$`, got[0].Name)
	assert.Equal(t, `!((request.userInfo.username != '') && (request.resource.resource == "accessentries"))`, got[0].Expression)

	other := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "other"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "other-condition",
				Expression: "object.metadata.namespace == 'skip'",
			}},
		},
	}
	got = negateExceptionMatchConditions([]policiesv1beta1.PolicyException{multi, other})
	require.Len(t, got, 2)
	assert.Equal(t, `!((request.userInfo.username != '') && (request.resource.resource == "accessentries"))`, got[0].Expression)
	assert.Equal(t, `!(object.metadata.namespace == 'skip')`, got[1].Expression)
}

func TestBuildValidatingAdmissionPolicy_MultiConditionCELException(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-standard-labels", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "not-deleting",
				Expression: "!has(object.metadata.deletionTimestamp)",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "true",
				Message:    "must pass",
			}},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "require-standard-labels-temp"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{
					Name:       "admission-only",
					Expression: `has(request.userInfo.username) && request.userInfo.username != ''`,
				},
				{
					Name:       "match-legacy-resources",
					Expression: `request.resource.resource == "accessentries"`,
				},
			},
		},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "vpol-require-standard-labels"},
	}

	err := BuildValidatingAdmissionPolicy(
		nil,
		vap,
		engineapi.NewValidatingPolicy(policy),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(exception)},
	)
	require.NoError(t, err)
	require.Len(t, vap.Spec.MatchConditions, 2)
	assert.Equal(t, "not-deleting", vap.Spec.MatchConditions[0].Name)
	assert.Equal(t, "!has(object.metadata.deletionTimestamp)", vap.Spec.MatchConditions[0].Expression)
	assert.Regexp(t, `^exception-[0-9a-f]{12}$`, vap.Spec.MatchConditions[1].Name)
	assert.Equal(t, `!((has(request.userInfo.username) && request.userInfo.username != '') && (request.resource.resource == "accessentries"))`, vap.Spec.MatchConditions[1].Expression)
	assert.Len(t, vap.Spec.Validations, 1)
	assert.Equal(t, "true", vap.Spec.Validations[0].Expression)
}

func TestNegateExceptionMatchConditions_FullAndScopedExceptions(t *testing.T) {
	tests := []struct {
		name      string
		exception policiesv1beta1.PolicyException
		wantCount int
		wantExpr  string
	}{
		{
			name: "full exception without match conditions",
			exception: policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "full"},
			},
			wantCount: 1,
			wantExpr:  "false",
		},
		{
			name: "image-scoped exception without match conditions",
			exception: policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "image-scoped"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					Images: []string{"ghcr.io/example/app:v1"},
				},
			},
		},
		{
			name: "value-scoped exception without match conditions",
			exception: policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "value-scoped"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					AllowedValues: []string{"allowed"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := negateExceptionMatchConditions([]policiesv1beta1.PolicyException{tt.exception})
			require.Len(t, got, tt.wantCount)
			if tt.wantCount == 0 {
				return
			}
			assert.Regexp(t, "^exception-[0-9a-f]{12}$", got[0].Name)
			assert.Equal(t, tt.wantExpr, got[0].Expression)
		})
	}
}

func TestNegateExceptionMatchConditions_MultipleConditionsEvaluation(t *testing.T) {
	exception := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "multi"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "admission-only", Expression: "request.userInfo.username != ''"},
				{Name: "match-legacy-resources", Expression: "request.resource.resource == \"accessentries\""},
			},
		},
	}
	generated := negateExceptionMatchConditions([]policiesv1beta1.PolicyException{exception})
	require.Len(t, generated, 1)

	env, err := cel.NewEnv(cel.Variable("request", cel.DynType))
	require.NoError(t, err)
	ast, issues := env.Compile(generated[0].Expression)
	require.NoError(t, issues.Err())
	program, err := env.Program(ast)
	require.NoError(t, err)

	tests := []struct {
		name    string
		request map[string]any
		want    bool
	}{
		{
			name: "only username condition matches; policy stays active",
			request: map[string]any{
				"userInfo": map[string]any{"username": "system:serviceaccount:kyverno:controller"},
				"resource": map[string]any{"resource": "pods"},
			},
			want: true,
		},
		{
			name: "both conditions match; policy is excluded",
			request: map[string]any{
				"userInfo": map[string]any{"username": "system:serviceaccount:kyverno:controller"},
				"resource": map[string]any{"resource": "accessentries"},
			},
			want: false,
		},
		{
			name: "only resource condition matches; policy stays active",
			request: map[string]any{
				"userInfo": map[string]any{"username": ""},
				"resource": map[string]any{"resource": "accessentries"},
			},
			want: true,
		},
		{
			name: "neither condition matches; policy stays active",
			request: map[string]any{
				"userInfo": map[string]any{"username": ""},
				"resource": map[string]any{"resource": "pods"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, _, err := program.Eval(map[string]any{"request": tt.request})
			require.NoError(t, err)
			assert.Equal(t, tt.want, out.Value())
		})
	}
}

func TestBuildValidatingAdmissionPolicy_DoesNotMutateCachedPolicy(t *testing.T) {
	// Keep a sentinel in spare capacity: appending an exception must not write
	// beyond the source slice's length, even if the source elements are unchanged.
	conditions := []admissionregistrationv1.MatchCondition{
		{Name: "scope", Expression: "exceptions.allowedImages.size() > 0"},
		{Name: "sentinel", Expression: "false"},
	}
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "cached-policy"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			MatchConditions:  conditions[:1],
			Variables: []admissionregistrationv1.Variable{{
				Name: "count", Expression: "exceptions.allowedImages.size()",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression:        "exceptions.allowedImages.size() > 0",
				MessageExpression: "string(exceptions.allowedValues.size())",
			}},
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
				Key: "scope", ValueExpression: "string(exceptions.allowedImages.size())",
			}},
		},
	}
	original := policy.DeepCopy()
	exception := &policiesv1beta1.PolicyException{
		Spec: policiesv1beta1.PolicyExceptionSpec{Images: []string{"example.com/app:v1"}},
	}
	fullException := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Name: "conditional-exemption"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{{Name: "skip", Expression: "false"}},
		},
	}
	var previous *admissionregistrationv1.ValidatingAdmissionPolicy
	for range 2 {
		vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
		err := BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(policy),
			[]engineapi.GenericException{engineapi.NewCELPolicyException(exception), engineapi.NewCELPolicyException(fullException)})
		require.NoError(t, err)
		assert.Equal(t, original, policy)
		assert.Equal(t, "sentinel", conditions[1].Name)
		assert.Equal(t, "false", conditions[1].Expression)
		if previous != nil {
			assert.Equal(t, previous, vap, "repeated reconciliation must be stable")
		}
		previous = vap
	}

	// Removing the exception must rebuild empty scopes from the original policy,
	// rather than keeping references or values from a previous reconciliation.
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	require.NoError(t, BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(policy), nil))
	assert.Equal(t, original, policy)
	assert.Equal(t, "[].size() > 0", vap.Spec.MatchConditions[0].Expression)
	require.Len(t, vap.Spec.Variables, 3)
	assert.Equal(t, "[]", vap.Spec.Variables[0].Expression)
	assert.Equal(t, "[]", vap.Spec.Variables[1].Expression)
}

func TestBuildValidatingAdmissionPolicy_InlinesMatchConditionScopes(t *testing.T) {
	tests := []struct {
		name   string
		images []string
		values []string
		want   bool
	}{
		{name: "empty scopes", want: false},
		{name: "images only", images: []string{"example.com/app:v1"}, want: true},
		{name: "values only", values: []string{"blue"}, want: false},
		{name: "images and values", images: []string{"example.com/app:v1"}, values: []string{"blue"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "match-condition-scopes"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{},
					MatchConditions: []admissionregistrationv1.MatchCondition{{
						Name: "scope", Expression: "exceptions.allowedImages.size() == 1 && exceptions.allowedValues.size() == 0",
					}},
					Validations: []admissionregistrationv1.Validation{{Expression: "true"}},
				},
			}
			var exceptions []engineapi.GenericException
			if len(tt.images)+len(tt.values) > 0 {
				exceptions = []engineapi.GenericException{engineapi.NewCELPolicyException(&policiesv1beta1.PolicyException{
					Spec: policiesv1beta1.PolicyExceptionSpec{Images: tt.images, AllowedValues: tt.values},
				})}
			}
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			require.NoError(t, BuildValidatingAdmissionPolicy(nil, vap, engineapi.NewValidatingPolicy(policy), exceptions))
			require.Len(t, vap.Spec.MatchConditions, 1)
			// Compile without either exceptions or variables in scope, as required by
			// the native admission match-condition environment.
			env, err := cel.NewEnv()
			require.NoError(t, err)
			ast, issues := env.Compile(vap.Spec.MatchConditions[0].Expression)
			require.NoError(t, issues.Err())
			program, err := env.Program(ast)
			require.NoError(t, err)
			out, _, err := program.Eval(map[string]any{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, out.Value())
		})
	}
}

func TestBuildValidatingAdmissionPolicy_FullCELException(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "require-standard-labels", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "true",
				Message:    "must pass",
			}},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "full-exception"},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}

	err := BuildValidatingAdmissionPolicy(
		nil,
		vap,
		engineapi.NewValidatingPolicy(policy),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(exception)},
	)
	require.NoError(t, err)
	require.Len(t, vap.Spec.MatchConditions, 1)
	assert.Regexp(t, "^exception-[0-9a-f]{12}$", vap.Spec.MatchConditions[0].Name)
	assert.Equal(t, "false", vap.Spec.MatchConditions[0].Expression)
	assert.Len(t, vap.Spec.Validations, 1)
}

func TestBuildMutatingAdmissionPolicyV1_FullCELException(t *testing.T) {
	mp := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mutate-policy", UID: "policy-uid"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
		},
	}
	exception := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "full-exception"},
	}
	mapol := &admissionregistrationv1.MutatingAdmissionPolicy{}

	BuildMutatingAdmissionPolicyV1(mapol, mp, []policiesv1beta1.PolicyException{exception})

	require.Len(t, mapol.Spec.MatchConditions, 1)
	assert.Regexp(t, "^exception-[0-9a-f]{12}$", mapol.Spec.MatchConditions[0].Name)
	assert.Equal(t, "false", mapol.Spec.MatchConditions[0].Expression)
}

func TestNegateExceptionMatchConditions_GlobalExceptionShortCircuits(t *testing.T) {
	exceptions := []policiesv1beta1.PolicyException{
		{ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "full-one"}},
		{ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "full-two"}},
	}

	got := negateExceptionMatchConditions(exceptions)
	require.Len(t, got, 1)
	assert.Equal(t, "false", got[0].Expression)
	assert.Regexp(t, "^exception-[0-9a-f]{12}$", got[0].Name)
}

func TestEnsureUniqueExceptionMatchConditionNames(t *testing.T) {
	exception := policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "collision"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			MatchConditions: []admissionregistrationv1.MatchCondition{
				{Name: "first", Expression: "true"},
				{Name: "second", Expression: "true"},
			},
		},
	}
	generated := negateExceptionMatchConditions([]policiesv1beta1.PolicyException{exception})
	require.Len(t, generated, 1)
	collidingName := generated[0].Name
	policyConditions := []admissionregistrationv1.MatchCondition{{
		Name:       collidingName,
		Expression: "true",
	}}

	got := ensureUniqueExceptionMatchConditionNames(generated, policyConditions)
	assert.Equal(t, collidingName+"-2", got[0].Name)
	assert.Equal(t, collidingName, policyConditions[0].Name)

	duplicateExceptionNames := []admissionregistrationv1.MatchCondition{
		{Name: "check-name", Expression: "true"},
		{Name: "check-name", Expression: "false"},
	}
	got = ensureUniqueExceptionMatchConditionNames(duplicateExceptionNames, nil)
	assert.Equal(t, "check-name", got[0].Name)
	assert.Equal(t, "check-name-2", got[1].Name)

	longName := strings.Repeat("a", 63)
	got = ensureUniqueExceptionMatchConditionNames(
		[]admissionregistrationv1.MatchCondition{{Name: longName, Expression: "true"}},
		[]admissionregistrationv1.MatchCondition{{Name: longName, Expression: "true"}},
	)
	require.Len(t, got, 1)
	assert.LessOrEqual(t, len(got[0].Name), 63)
	assert.Equal(t, strings.Repeat("a", 61)+"-2", got[0].Name)
}

func TestEnsureUniqueExceptionMatchConditionNames_MaxLength(t *testing.T) {
	longName := strings.Repeat("a", 63)
	existing := []admissionregistrationv1.MatchCondition{{Name: longName, Expression: "true"}}
	generated := []admissionregistrationv1.MatchCondition{{Name: longName, Expression: "false"}}

	got := ensureUniqueExceptionMatchConditionNames(generated, existing)
	require.Len(t, got, 1)
	assert.Len(t, got[0].Name, 63)
	assert.Equal(t, strings.Repeat("a", 61)+"-2", got[0].Name)
	assert.Equal(t, longName, existing[0].Name)
}

func TestBuildValidatingAdmissionPolicy_RewritesAllScopedExceptionFields(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "scoped-references", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Variables: []admissionregistrationv1.Variable{{
				Name:       "scopeCount",
				Expression: "exceptions.allowedImages.size() + exceptions.allowedValues.size()",
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression:        "exceptions.allowedImages.size() > 0",
				MessageExpression: "'allowed: ' + string(exceptions.allowedValues.size())",
			}},
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
				Key:             "example.com/image-count",
				ValueExpression: "string(exceptions.allowedImages.size())",
			}},
		},
	}
	exception := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "only-images"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/app:v1"},
		},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
	err := BuildValidatingAdmissionPolicy(
		nil, vap, engineapi.NewValidatingPolicy(policy),
		[]engineapi.GenericException{engineapi.NewCELPolicyException(exception)},
	)
	require.NoError(t, err)
	require.Len(t, vap.Spec.Variables, 3)
	variableExpressions := make(map[string]string, len(vap.Spec.Variables))
	for _, variable := range vap.Spec.Variables {
		variableExpressions[variable.Name] = variable.Expression
	}
	assert.Equal(t, "variables.allowedImages.size() + variables.allowedValues.size()", variableExpressions["scopeCount"])
	assert.Equal(t, `["ghcr.io/example/app:v1"]`, variableExpressions["allowedImages"])
	assert.Equal(t, "[]", variableExpressions["allowedValues"])
	require.Len(t, vap.Spec.Validations, 1)
	assert.Equal(t, "variables.allowedImages.size() > 0", vap.Spec.Validations[0].Expression)
	assert.Equal(t, `"allowed: " + string(variables.allowedValues.size())`, vap.Spec.Validations[0].MessageExpression)
	require.Len(t, vap.Spec.AuditAnnotations, 1)
	assert.Equal(t, "string(variables.allowedImages.size())", vap.Spec.AuditAnnotations[0].ValueExpression)
}

func TestBuildValidatingAdmissionPolicy_AggregatesScopedExceptionValues(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "scoped-exceptions", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Variables: []admissionregistrationv1.Variable{{
				Name:       "allowedImages",
				Expression: "exceptions.allowedValues.size() > 0",
			}},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression:        "exceptions.allowedImages.size() > 0",
					Message:           "image list must exist",
					MessageExpression: "exceptions.allowedValues.join(',')",
				},
				{Expression: "exceptions.allowedValues.size() > 0", Message: "value list must exist"},
			},
			AuditAnnotations: []admissionregistrationv1.AuditAnnotation{{
				Key:             "exception-images",
				ValueExpression: "exceptions.allowedImages.join(',')",
			}},
		},
	}
	imageOne := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "image-one"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images:        []string{"ghcr.io/example/one:v1"},
			AllowedValues: []string{"blue"},
		},
	}
	imageTwo := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "image-two"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images:        []string{"ghcr.io/example/two:v1"},
			AllowedValues: []string{"green"},
		},
	}
	vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}

	err := BuildValidatingAdmissionPolicy(
		nil,
		vap,
		engineapi.NewValidatingPolicy(policy),
		[]engineapi.GenericException{
			engineapi.NewCELPolicyException(imageOne),
			engineapi.NewCELPolicyException(imageTwo),
		},
	)
	require.NoError(t, err)

	require.Len(t, vap.Spec.Variables, 3)
	variables := make(map[string]string, len(vap.Spec.Variables))
	for _, variable := range vap.Spec.Variables {
		variables[variable.Name] = variable.Expression
	}
	assert.Equal(t, "variables.allowedValues.size() > 0", variables["allowedImages"])
	assert.Equal(t, `["ghcr.io/example/one:v1","ghcr.io/example/two:v1"]`, variables["exceptionAllowedImages2"])
	assert.Equal(t, `["blue","green"]`, variables["allowedValues"])
	assert.Equal(t, "variables.exceptionAllowedImages2.size() > 0", vap.Spec.Validations[0].Expression)
	assert.Equal(t, "variables.allowedValues.size() > 0", vap.Spec.Validations[1].Expression)
	assert.Equal(t, `variables.allowedValues.join(",")`, vap.Spec.Validations[0].MessageExpression)
	require.Len(t, vap.Spec.AuditAnnotations, 1)
	assert.Equal(t, `variables.exceptionAllowedImages2.join(",")`, vap.Spec.AuditAnnotations[0].ValueExpression)
}

func TestBuildValidatingAdmissionPolicy_EmptyScopedExceptionValues(t *testing.T) {
	tests := []struct {
		name             string
		exception        *policiesv1beta1.PolicyException
		wantImages       string
		wantValues       string
		wantExpression   string
		sourceExpression string
	}{
		{
			name: "image-only exception declares empty allowed values",
			exception: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "image-only"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					Images: []string{"ghcr.io/example/app:v1"},
				},
			},
			wantImages:       `["ghcr.io/example/app:v1"]`,
			wantValues:       "[]",
			wantExpression:   "variables.allowedImages.size() > 0 && variables.allowedValues.size() == 0",
			sourceExpression: "exceptions.allowedImages.size() > 0 && exceptions.allowedValues.size() == 0",
		},
		{
			name: "value-only exception declares empty allowed images",
			exception: &policiesv1beta1.PolicyException{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kyverno", Name: "value-only"},
				Spec: policiesv1beta1.PolicyExceptionSpec{
					AllowedValues: []string{"blue"},
				},
			},
			wantImages:       "[]",
			wantValues:       `["blue"]`,
			wantExpression:   "variables.allowedImages.size() == 0 && variables.allowedValues.size() > 0",
			sourceExpression: "exceptions.allowedImages.size() == 0 && exceptions.allowedValues.size() > 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &policiesv1beta1.ValidatingPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-scoped-exceptions"},
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{},
					Validations: []admissionregistrationv1.Validation{{
						Expression: tt.sourceExpression,
						Message:    "scope test",
					}},
				},
			}
			vap := &admissionregistrationv1.ValidatingAdmissionPolicy{}
			err := BuildValidatingAdmissionPolicy(
				nil,
				vap,
				engineapi.NewValidatingPolicy(policy),
				[]engineapi.GenericException{engineapi.NewCELPolicyException(tt.exception)},
			)
			require.NoError(t, err)
			require.Len(t, vap.Spec.Variables, 2)
			variables := make(map[string]string, len(vap.Spec.Variables))
			for _, variable := range vap.Spec.Variables {
				variables[variable.Name] = variable.Expression
			}
			assert.Equal(t, tt.wantImages, variables["allowedImages"])
			assert.Equal(t, tt.wantValues, variables["allowedValues"])
			assert.Equal(t, tt.wantExpression, vap.Spec.Validations[0].Expression)
		})
	}
}

func TestBuildValidatingAdmissionPolicy_DeterministicExceptionScopeOrder(t *testing.T) {
	policy := &policiesv1beta1.ValidatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sorted-exceptions", UID: "policy-uid"},
		Spec: policiesv1beta1.ValidatingPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "exceptions.allowedImages.size() > 0 && exceptions.allowedValues.size() > 0",
				Message:    "scopes must exist",
			}},
		},
	}
	alpha := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "a-namespace", Name: "z-exception"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/alpha:v1"},
			AllowedValues: []string{"alpha"},
		},
	}
	beta := &policiesv1beta1.PolicyException{
		ObjectMeta: metav1.ObjectMeta{Namespace: "b-namespace", Name: "a-exception"},
		Spec: policiesv1beta1.PolicyExceptionSpec{
			Images: []string{"ghcr.io/example/beta:v1"},
			AllowedValues: []string{"beta"},
		},
	}
	build := func(first, second *policiesv1beta1.PolicyException) admissionregistrationv1.ValidatingAdmissionPolicySpec {
		t.Helper()
		var vap admissionregistrationv1.ValidatingAdmissionPolicy
		err := BuildValidatingAdmissionPolicy(
			nil, &vap, engineapi.NewValidatingPolicy(policy),
			[]engineapi.GenericException{engineapi.NewCELPolicyException(first), engineapi.NewCELPolicyException(second)},
		)
		require.NoError(t, err)
		return vap.Spec
	}
	inOrder := build(alpha, beta)
	reversed := build(beta, alpha)
	assert.Equal(t, inOrder, reversed)
	require.Len(t, inOrder.Variables, 2)
	assert.Equal(t, `["ghcr.io/example/alpha:v1","ghcr.io/example/beta:v1"]`, inOrder.Variables[0].Expression)
	assert.Equal(t, `["alpha","beta"]`, inOrder.Variables[1].Expression)
}
