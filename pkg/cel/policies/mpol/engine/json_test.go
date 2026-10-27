package engine

import (
	"context"
	"strings"
	"testing"

	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celengine "github.com/kyverno/kyverno/pkg/cel/engine"
	"github.com/kyverno/kyverno/pkg/cel/matching"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/kyverno/kyverno/pkg/cel/requestbudget"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	kjson "sigs.k8s.io/json"
)

func jsonMutationPolicy(name string, expressions ...string) *policiesv1beta1.MutatingPolicy {
	policy := &policiesv1beta1.MutatingPolicy{
		TypeMeta:   metav1.TypeMeta{Kind: "MutatingPolicy", APIVersion: "policies.kyverno.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: policiesv1beta1.MutatingPolicySpec{
			EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{Mode: policieskyvernoio.EvaluationModeJSON},
		},
	}
	for _, expression := range expressions {
		policy.Spec.Mutations = append(policy.Spec.Mutations, admissionregistrationv1alpha1.Mutation{
			PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
			JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression},
		})
	}
	return policy
}

func jsonObject(t *testing.T, raw string) *unstructured.Unstructured {
	t.Helper()
	var object map[string]any
	require.NoError(t, kjson.UnmarshalCaseSensitivePreserveInts([]byte(raw), &object))
	return &unstructured.Unstructured{Object: object}
}

func TestJSONMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, expression, expected string
		status                            engineapi.RuleStatus
	}{
		{"mixed values", `{}`, `[JSONPatch{op:"add",path:"/data",value:{"list":[1,"two",null,true]}}]`, `{"data":{"list":[1,"two",null,true]}}`, engineapi.RuleStatusPass},
		{"nested array", `{"items":[1,2]}`, `[JSONPatch{op:"add",path:"/items/-",value:3}]`, `{"items":[1,2,3]}`, engineapi.RuleStatusPass},
		{"escaped key", `{}`, `[JSONPatch{op:"add",path:"/"+jsonpatch.escapeKey("a/b~c"),value:true}]`, `{"a/b~c":true}`, engineapi.RuleStatusPass},
		{"copy and remove", `{"a":1}`, `[JSONPatch{op:"copy",from:"/a",path:"/b"},JSONPatch{op:"remove",path:"/a"}]`, `{"b":1}`, engineapi.RuleStatusPass},
		{"large integer", `{"id":9007199254740993}`, `[JSONPatch{op:"add",path:"/copy",value:object.id}]`, `{"id":9007199254740993,"copy":9007199254740993}`, engineapi.RuleStatusPass},
		{"fractional equality", `{"n":0.1}`, `[JSONPatch{op:"test",path:"/n",value:0.1},JSONPatch{op:"add",path:"/ok",value:true}]`, `{"n":0.1,"ok":true}`, engineapi.RuleStatusPass},
		{"mixed numeric inequality rolls back", `{"n":9007199254740993}`, `[JSONPatch{op:"add",path:"/partial",value:true},JSONPatch{op:"test",path:"/n",value:9007199254740992.0}]`, `{"n":9007199254740993}`, engineapi.RuleStatusSkip},
		{"invalid pointer", `{}`, `[JSONPatch{op:"add",path:"/bad~2key",value:true}]`, "", engineapi.RuleStatusError},
		{"root type change", `{}`, `[JSONPatch{op:"replace",path:"",value:[1,2]}]`, "", engineapi.RuleStatusError},
		{"null root", `{}`, `[JSONPatch{op:"replace",path:"",value:null}]`, "", engineapi.RuleStatusError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{jsonMutationPolicy("test", tc.expression)}, nil, nil)
			require.NoError(t, err)
			engine := NewEngine(provider, nil, nil, nil, nil)
			input := jsonObject(t, tc.input)
			original := input.DeepCopy()
			response, err := engine.Handle(context.Background(), celengine.RequestFromJSON(nil, input), nil)
			require.Equal(t, original, input)
			require.Len(t, response.Policies, 1)
			require.Equal(t, tc.status, response.Policies[0].Rules[0].Status())
			if tc.status == engineapi.RuleStatusError {
				require.Error(t, err)
				require.Nil(t, response.PatchedResource)
			} else {
				require.NoError(t, err)
				require.Equal(t, jsonObject(t, tc.expected), response.PatchedResource)
			}
		})
	}
}

func TestJSONMutationReusesProviderAndSnapshots(t *testing.T) {
	t.Parallel()
	first := jsonMutationPolicy("first",
		`[JSONPatch{op:"add",path:"/count",value:1}]`,
		`[JSONPatch{op:"replace",path:"/count",value:variables.count+1}]`,
	)
	first.Spec.Variables = []admissionregistrationv1.Variable{{Name: "count", Expression: "object.count"}}
	second := jsonMutationPolicy("second", `[JSONPatch{op:"add",path:"/result",value:object.count*2}]`)
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{first, second}, nil, nil)
	require.NoError(t, err)
	first.Spec.Mutations[0].JSONPatch.Expression = "invalid("
	engine := NewEngine(provider, nil, nil, nil, nil)
	for range 2 {
		response, err := engine.Handle(context.Background(), celengine.RequestFromJSON(nil, jsonObject(t, `{}`)), nil)
		require.NoError(t, err)
		require.Equal(t, jsonObject(t, `{"count":2,"result":4}`), response.PatchedResource)
		require.Equal(t, jsonObject(t, `{"count":2}`), response.Policies[0].PatchedResource)
		response.PatchedResource.Object["count"] = int64(99)
		require.Equal(t, int64(2), response.Policies[0].PatchedResource.Object["count"])
	}
}

func TestJSONMutationLimits(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/ok",value:true}]`)
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, nil)
	require.NoError(t, err)
	engine := NewEngine(provider, nil, nil, nil, nil)
	t.Run("caller budget", func(t *testing.T) {
		t.Parallel()
		ctx, _ := requestbudget.Ensure(context.Background())
		require.ErrorIs(t, requestbudget.Charge(ctx, ^uint64(0)), requestbudget.ErrBudgetExceeded)
		response, err := engine.Handle(ctx, celengine.RequestFromJSON(nil, jsonObject(t, `{}`)), nil)
		require.Error(t, err)
		require.Nil(t, response.PatchedResource)
	})
	t.Run("cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		response, err := engine.Handle(ctx, celengine.RequestFromJSON(nil, jsonObject(t, `{}`)), nil)
		require.Error(t, err)
		require.Nil(t, response.PatchedResource)
	})
	t.Run("size", func(t *testing.T) {
		t.Parallel()
		input := &unstructured.Unstructured{Object: map[string]any{"value": strings.Repeat("x", compiler.MaxJSONDocumentBytes)}}
		response, err := engine.Handle(context.Background(), celengine.RequestFromJSON(nil, input), nil)
		require.ErrorContains(t, err, "exceeds")
		require.Nil(t, response.PatchedResource)
	})
}

func TestJSONMutationAdmissionUsesNormalMatching(t *testing.T) {
	t.Parallel()
	policy := jsonMutationPolicy("test", `[JSONPatch{op:"add",path:"/ok",value:true}]`)
	policy.Spec.MatchConstraints = &admissionregistrationv1.MatchResources{
		ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
			RuleWithOperations: admissionregistrationv1.RuleWithOperations{
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
				Rule:       admissionregistrationv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}},
			},
		}},
	}
	provider, err := NewProvider(compiler.NewCompiler(), []policiesv1beta1.MutatingPolicyLike{policy}, nil, nil)
	require.NoError(t, err)
	engine := NewEngine(provider, nil, matching.NewMatcher(), nil, nil)
	request := celengine.EngineRequest{Request: admissionv1.AdmissionRequest{
		Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		Resource:  metav1.GroupVersionResource{Version: "v1", Resource: "configmaps"},
		Name:      "input",
		Operation: admissionv1.Create,
		Object:    runtime.RawExtension{Raw: []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"input"}}`)},
	}}
	response, err := engine.Handle(context.Background(), request, nil)
	require.NoError(t, err)
	require.Len(t, response.Policies, 1)
	require.Equal(t, engineapi.RuleStatusPass, response.Policies[0].Rules[0].Status())
	require.Equal(t, true, response.PatchedResource.Object["ok"])
	request.Request.Operation = admissionv1.Update
	response, err = engine.Handle(context.Background(), request, nil)
	require.NoError(t, err)
	require.Empty(t, response.Policies[0].Rules)
	require.Nil(t, response.PatchedResource)
}
