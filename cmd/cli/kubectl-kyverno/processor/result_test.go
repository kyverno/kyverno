package processor

import (
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAddGenerateResponse_GeneratingPolicy(t *testing.T) {
	t.Parallel()

	gpol := &policiesv1beta1.GeneratingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "example"},
	}
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.io/v1alpha1",
			"kind":       "MyResource",
			"metadata": map[string]interface{}{
				"name":      "my-resource",
				"namespace": "default",
			},
		},
	}

	tests := []struct {
		name     string
		rule     engineapi.RuleResponse
		expected ResultCounts
	}{
		{
			name: "error",
			rule: *engineapi.RuleError("example", engineapi.Generation, "failed to evaluate policy", errors.New("no such key: uid"), nil),
			expected: ResultCounts{
				Error: 1,
			},
		},
		{
			name: "skip",
			rule: *engineapi.RuleSkip("example", engineapi.Generation, "skipped", nil),
			expected: ResultCounts{
				Skip: 1,
			},
		},
		{
			name: "pass",
			rule: *engineapi.RulePass("example", engineapi.Generation, "policy evaluated successfully", nil),
			expected: ResultCounts{
				Pass: 1,
			},
		},
		{
			name: "fail",
			rule: *engineapi.RuleFail("example", engineapi.Generation, "generation failed", nil),
			expected: ResultCounts{
				Fail: 1,
			},
		},
		{
			name: "warn",
			rule: *engineapi.RuleWarn("example", engineapi.Generation, "warning", nil),
			expected: ResultCounts{
				Warn: 1,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rc := &ResultCounts{}
			response := engineapi.NewEngineResponse(
				resource,
				engineapi.NewGeneratingPolicy(gpol),
				nil,
			).WithPolicyResponse(engineapi.PolicyResponse{
				Rules: []engineapi.RuleResponse{tt.rule},
			})
			rc.addGenerateResponse(response)
			assert.Equal(t, tt.expected.Pass, rc.Pass)
			assert.Equal(t, tt.expected.Fail, rc.Fail)
			assert.Equal(t, tt.expected.Warn, rc.Warn)
			assert.Equal(t, tt.expected.Error, rc.Error)
			assert.Equal(t, tt.expected.Skip, rc.Skip)
		})
	}
}
