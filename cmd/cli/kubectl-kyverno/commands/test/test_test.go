package test

import (
	"bytes"
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func TestConvertNumericValuesToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "int to float64",
			input:    10,
			expected: 10.0,
		},
		{
			name:     "int32 to float64",
			input:    int32(20),
			expected: 20.0,
		},
		{
			name:     "int64 to float64",
			input:    int64(30),
			expected: 30.0,
		},
		{
			name:     "map with mixed values",
			input:    map[string]interface{}{"a": 5, "b": "string", "c": int32(10)},
			expected: map[string]interface{}{"a": 5.0, "b": "string", "c": 10.0},
		},
		{
			name:     "nested map",
			input:    map[string]interface{}{"a": map[string]interface{}{"b": int64(40)}},
			expected: map[string]interface{}{"a": map[string]interface{}{"b": 40.0}},
		},
		{
			name:     "slice of ints",
			input:    []interface{}{1, int32(2), int64(3), "text"},
			expected: []interface{}{1.0, 2.0, 3.0, "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertNumericValuesToFloat64(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestProcessResources(t *testing.T) {
	resources := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": ptr.To(int32(3)),
				},
			},
		},
	}

	expected := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"replicas": 3.0,
				},
			},
		},
	}

	processed := ProcessResources(resources)

	if !reflect.DeepEqual(processed, expected) {
		t.Errorf("Expected %v, got %v", expected, processed)
	}
}

func TestValidatePolicies(t *testing.T) {
	t.Parallel()

	deprecatedPolicy := &kyvernov1.Policy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deprecated",
			Namespace: "default",
		},
		Spec: kyvernov1.Spec{
			ValidationFailureAction: "enforce", // lowercase is deprecated
		},
	}

	invalidPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-cluster-policy",
			Namespace: "not-allowed-on-cluster-policy",
		},
	}

	t.Run("empty policies", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		valid, skipped, err := validatePolicies(&out, nil, false)
		assert.NoError(t, err)
		assert.Empty(t, valid)
		assert.Empty(t, skipped)
		assert.Empty(t, out.String())
	})

	t.Run("policy with deprecation warning", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		valid, skipped, err := validatePolicies(&out, []kyvernov1.PolicyInterface{deprecatedPolicy}, false)
		assert.NoError(t, err)
		assert.Len(t, valid, 1)
		assert.Empty(t, skipped)
		assert.Contains(t, out.String(), "Warning: policy default/test-deprecated:")
		assert.Contains(t, out.String(), "Validation failure actions enforce/audit are deprecated")
	})

	t.Run("policy with deprecation warning and failOnWarnings", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		valid, skipped, err := validatePolicies(&out, []kyvernov1.PolicyInterface{deprecatedPolicy}, true)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "found 1 deprecation warning(s) and --warnings-as-errors is set")
		assert.Empty(t, valid)
		assert.Empty(t, skipped)
		assert.Contains(t, out.String(), "Warning: policy default/test-deprecated:")
		assert.Contains(t, out.String(), "Validation failure actions enforce/audit are deprecated")
	})

	t.Run("invalid policy is skipped and duplicate warning is suppressed", func(t *testing.T) {
		t.Parallel()
		var out bytes.Buffer
		valid, skipped, err := validatePolicies(&out, []kyvernov1.PolicyInterface{invalidPolicy}, false)
		assert.NoError(t, err)
		assert.Empty(t, valid)
		assert.Contains(t, skipped, "invalid-cluster-policy")
		assert.Empty(t, out.String())
	})

	t.Run("valid policy with no warnings", func(t *testing.T) {
		t.Parallel()
		validPolicy := &kyvernov1.ClusterPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name: "valid-policy",
			},
			Spec: kyvernov1.Spec{
				Rules: []kyvernov1.Rule{
					{
						Name: "rule-1",
						MatchResources: kyvernov1.MatchResources{
							ResourceDescription: kyvernov1.ResourceDescription{
								Kinds: []string{"Pod"},
							},
						},
						Validation: &kyvernov1.Validation{
							Message: "label 'app.kubernetes.io/name' is required",
							RawPattern: &apiextensionsv1.JSON{
								Raw: []byte(`{"metadata":{"labels":{"app.kubernetes.io/name":"?*"}}}`),
							},
						},
					},
				},
			},
		}
		var out bytes.Buffer
		valid, skipped, err := validatePolicies(&out, []kyvernov1.PolicyInterface{validPolicy}, false)
		assert.NoError(t, err)
		assert.Len(t, valid, 1)
		assert.Empty(t, skipped)
		assert.Empty(t, out.String())
	})
}
