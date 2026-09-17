package compiler

import (
	"context"
	"testing"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
)

// compileJSONPatcher compiles a MutatingPolicy carrying a single JSONPatch mutation and
// returns its patcher, so the expressions below travel the same path they do in the
// admission and reports controllers.
func compileJSONPatcher(t *testing.T, expression string) *jsonPatcher {
	t.Helper()
	policy := &v1beta1.MutatingPolicy{
		Spec: v1beta1.MutatingPolicySpec{
			Mutations: []admissionregistrationv1alpha1.Mutation{{
				PatchType: admissionregistrationv1alpha1.PatchTypeJSONPatch,
				JSONPatch: &admissionregistrationv1alpha1.JSONPatch{Expression: expression},
			}},
		},
	}
	compiled, errs := NewCompiler().Compile(policy, nil)
	require.Nil(t, errs, "expression should compile")
	require.Len(t, compiled.patchers, 1)
	patcher, ok := compiled.patchers[0].(*jsonPatcher)
	require.True(t, ok, "expected a JSON patcher, got %T", compiled.patchers[0])
	return patcher
}

func jsonPatchEvalData() map[string]any {
	return map[string]any{
		"object":          map[string]any{"spec": map[string]any{}},
		"oldObject":       nil,
		"request":         nil,
		"namespaceObject": nil,
		"variables":       map[string]any{},
	}
}

// evalPatchValue evaluates the expression and returns the JSON encoding of the "value"
// member of its single operation.
func evalPatchValue(t *testing.T, expression string) (string, error) {
	t.Helper()
	patcher := compileJSONPatcher(t, expression)
	patch, _, err := patcher.evaluatePatchExpression(context.Background(), 10000000, jsonPatchEvalData())
	if err != nil {
		return "", err
	}
	require.Len(t, patch, 1)
	value, ok := patch[0]["value"]
	require.True(t, ok, "operation has no value member")
	return string(*value), nil
}

func TestEvaluatePatchExpressionValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{{
		// https://github.com/kyverno/kyverno/issues/17611: converting a list holding an
		// Object initializer used to panic in cel-go and take the process down with it.
		name: "list of object initializers",
		value: `[
			Object.spec.tolerations{ key: "kubernetes.azure.com/scalesetpriority", effect: "NoSchedule" },
			Object.spec.tolerations{ key: "second", effect: "NoExecute" }
		]`,
		want: `[{"effect":"NoSchedule","key":"kubernetes.azure.com/scalesetpriority"},{"effect":"NoExecute","key":"second"}]`,
	}, {
		name:  "bare object initializer",
		value: `Object.spec.tolerations{ key: "kubernetes.azure.com/scalesetpriority", effect: "NoSchedule" }`,
		want:  `{"effect":"NoSchedule","key":"kubernetes.azure.com/scalesetpriority"}`,
	}, {
		name:  "object initializer nested in an object",
		value: `Object.spec{ tolerations: [Object.spec.tolerations{ key: "k" }] }`,
		want:  `{"tolerations":[{"key":"k"}]}`,
	}, {
		// A map reaches the same defect by the other reflective assignment in cel-go,
		// SetMapIndex rather than Set, so it needs its own coverage.
		name:  "map with an object initializer value",
		value: `{"toleration": Object.spec.tolerations{ key: "k" }}`,
		want:  `{"toleration":{"key":"k"}}`,
	}, {
		name:  "map holding a list of object initializers",
		value: `{"tolerations": [Object.spec.tolerations{ key: "k" }]}`,
		want:  `{"tolerations":[{"key":"k"}]}`,
	}, {
		name:  "object holding a map",
		value: `Object.spec{ selector: {"app": "x"} }`,
		want:  `{"selector":{"app":"x"}}`,
	}, {
		name:  "string",
		value: `"Always"`,
		want:  `"Always"`,
	}, {
		name:  "int",
		value: `3`,
		want:  `3`,
	}, {
		name:  "bool",
		value: `true`,
		want:  `true`,
	}, {
		name:  "list of scalars",
		value: `["a", "b"]`,
		want:  `["a","b"]`,
	}, {
		name:  "empty list",
		value: `[]`,
		want:  `[]`,
	}, {
		name:  "nested list of scalars",
		value: `[["a"], ["b"]]`,
		want:  `[["a"],["b"]]`,
	}, {
		name:  "map",
		value: `{"annotated": "yes"}`,
		want:  `{"annotated":"yes"}`,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evalPatchValue(t, `[JSONPatch{op: "add", path: "/spec/x", value: `+tt.value+`}]`)
			assert.NoError(t, err)
			assert.JSONEq(t, tt.want, got)
		})
	}
}

// The type-name sanity check compares the type name of a nested Object initializer with
// the field path that reaches it, so it only ever reports a nested mismatch: the value at
// the root of the check is the start of that path and cannot disagree with itself.
// Encoding a list element by element makes the check reachable for the objects inside it,
// which it was not before, since such a value panicked during conversion instead.
func TestEvaluatePatchExpressionTypeNameMismatch(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		wantError bool
	}{{
		name:      "mismatch nested in an object",
		value:     `Object.spec{ tolerations: [Object.spec.wrong{ key: "k" }] }`,
		wantError: true,
	}, {
		name:      "mismatch nested in an object inside a list",
		value:     `[Object.spec.tolerations{ selector: Object.spec.wrong{ key: "k" } }]`,
		wantError: true,
	}, {
		name:      "object at the root of the check cannot mismatch",
		value:     `Object.spec.wrong{ key: "k" }`,
		wantError: false,
	}, {
		name:      "list element is its own root and cannot mismatch",
		value:     `[Object.spec.wrong{ key: "k" }]`,
		wantError: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evalPatchValue(t, `[JSONPatch{op: "add", path: "/spec/tolerations", value: `+tt.value+`}]`)
			if !tt.wantError {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "type mismatch")
		})
	}
}

// An operation without a value must not gain one, and "from" is carried through.
func TestEvaluatePatchExpressionOperationMembers(t *testing.T) {
	patcher := compileJSONPatcher(t, `[JSONPatch{op: "move", from: "/spec/a", path: "/spec/b"}]`)
	patch, _, err := patcher.evaluatePatchExpression(context.Background(), 10000000, jsonPatchEvalData())
	require.NoError(t, err)
	require.Len(t, patch, 1)
	assert.JSONEq(t, `"move"`, string(*patch[0]["op"]))
	assert.JSONEq(t, `"/spec/a"`, string(*patch[0]["from"]))
	assert.JSONEq(t, `"/spec/b"`, string(*patch[0]["path"]))
	_, hasValue := patch[0]["value"]
	assert.False(t, hasValue, "a move operation should carry no value")
}
