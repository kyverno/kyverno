package policy

import (
	"encoding/json"
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

func TestToJson(t *testing.T) {
	tests := []struct {
		name   string
		policy kyvernov1.PolicyInterface
	}{
		{
			name: "Valid ClusterPolicy",
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
				TypeMeta:   metav1.TypeMeta{Kind: "APIv1"},
			},
		},
		{
			name: "Valid Policy",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-namespace-policy", Namespace: "test-namespace"},
			},
		},
		{
			name:   "Nil Policy",
			policy: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ToJson(test.policy)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			expectedResult, err := json.Marshal(test.policy)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, expectedResult) {
				t.Errorf("Expected Json %s, got %s", expectedResult, result)
			}
		})
	}
}

func TestToYaml(t *testing.T) {
	tests := []struct {
		name   string
		policy kyvernov1.PolicyInterface
	}{
		{
			name: "Valid ClusterPolicy",
			policy: &kyvernov1.ClusterPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
			},
		},
		{
			name: "Valid Policy",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{Name: "test-namespace-policy", Namespace: "test-namespace"},
			},
		},
		{
			name:   "Nil Policy",
			policy: nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := ToYaml(test.policy)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			jsonBytes, err := ToJson(test.policy)
			if err != nil {
				if !reflect.DeepEqual(result, nil) {
					t.Errorf("Expected Json %+v, got %s", nil, result)
				}
			}
			expectedResult, err := yaml.JSONToYAML(jsonBytes)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, expectedResult) {
				t.Errorf("Expected Json %s, got %s", expectedResult, result)
			}
		})
	}
}
