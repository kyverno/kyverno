package webhook

import (
	"cmp"
	"reflect"
	"slices"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestAddOperationsForValidatingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyverno.Rule
		expectedResult map[string][]admissionregistrationv1.OperationType
	}{
		{
			name: "Test Case 1",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"ConfigMap"},
							Operations: []kyverno.AdmissionOperation{"CREATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"ConfigMap": {"CREATE"},
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"ConfigMap"},
						},
					},
					ExcludeResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []kyverno.AdmissionOperation{"DELETE", "CONNECT", "CREATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"ConfigMap": {"UPDATE"},
			},
		},
		{
			name: "Test Case 3",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"ConfigMap"},
							Operations: []kyverno.AdmissionOperation{"CREATE"},
						},
					},
				},
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"ConfigMap"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"ConfigMap": {"CREATE", "UPDATE", "DELETE", "CONNECT"},
			},
		},
		{
			name: "Test Case 4",
			rules: []kyverno.Rule{
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"ConfigMap"},
							Operations: []kyverno.AdmissionOperation{"CREATE"},
						},
					},
				},
				{
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"ConfigMap"},
							Operations: []kyverno.AdmissionOperation{"UPDATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"ConfigMap": {"CREATE", "UPDATE"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			result = addOpnForValidatingWebhookConf(testCase.rules, mapResourceToOpnType)

			for key, expectedValue := range testCase.expectedResult {
				slices.SortFunc(expectedValue, func(a, b admissionregistrationv1.OperationType) int {
					return cmp.Compare(a, b)
				})
				value := result[key]
				slices.SortFunc(value, func(a, b admissionregistrationv1.OperationType) int {
					return cmp.Compare(a, b)
				})
				if !reflect.DeepEqual(expectedValue, value) {
					t.Errorf("key: %v, expected %v, but got %v", key, expectedValue, value)
				}
			}
		})
	}
}

func TestAddOperationsForMutatingtingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyverno.Rule
		expectedResult map[string][]admissionregistrationv1.OperationType
	}{
		{
			name: "Test Case 1",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"ConfigMap"},
							Operations: []kyverno.AdmissionOperation{"CREATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"ConfigMap": {"CREATE"},
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Secret"},
						},
					},
					ExcludeResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []kyverno.AdmissionOperation{"UPDATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"Secret": {"CREATE"},
			},
		},
		{
			name: "Test Case 3",
			rules: []kyverno.Rule{
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"Secret"},
							Operations: []kyverno.AdmissionOperation{"CREATE"},
						},
					},
				},
				{
					Mutation: kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds:      []string{"Secret"},
							Operations: []kyverno.AdmissionOperation{"UPDATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"Secret": {"CREATE", "UPDATE"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			result = addOpnForMutatingWebhookConf(testCase.rules, mapResourceToOpnType)

			for key, expectedValue := range testCase.expectedResult {
				slices.SortFunc(expectedValue, func(a, b admissionregistrationv1.OperationType) int {
					return cmp.Compare(a, b)
				})
				value := result[key]
				slices.SortFunc(value, func(a, b admissionregistrationv1.OperationType) int {
					return cmp.Compare(a, b)
				})
				if !reflect.DeepEqual(expectedValue, value) {
					t.Errorf("key: %v, expected %v, but got %v", key, expectedValue, value)
				}
			}
		})
	}
}
