package webhook

import (
	"reflect"
	"sort"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func TestGetMinimumOperations(t *testing.T) {
	testCases := []struct {
		name           string
		inputMap       map[string]bool
		expectedResult []admissionregistrationv1.OperationType
	}{
		{
			name: "Test Case 1",
			inputMap: map[string]bool{
				"CREATE": true,
				"UPDATE": false,
				"DELETE": true,
			},
			expectedResult: []admissionregistrationv1.OperationType{"CREATE", "DELETE"},
		},
		{
			name: "Test Case 2",
			inputMap: map[string]bool{
				"CREATE":  false,
				"UPDATE":  false,
				"DELETE":  false,
				"CONNECT": true,
			},
			expectedResult: []admissionregistrationv1.OperationType{"CONNECT"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := getMinimumOperations(testCase.inputMap)
			sort.Slice(result, func(i, j int) bool {
				return result[i] < result[j]
			})
			sort.Slice(testCase.expectedResult, func(i, j int) bool {
				return testCase.expectedResult[i] < testCase.expectedResult[j]
			})

			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}

func TestComputeOperationsForMutatingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyvernov1.Rule
		expectedResult map[string]bool
	}{
		{
			name: "Test Case 1",
			rules: []kyvernov1.Rule{
				{
					Mutation: kyvernov1.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Operations: []v1.AdmissionOperation{"CREATE"},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				"CREATE": true,
			},
		},
		{
			name: "Test Case 2",
			rules: []kyvernov1.Rule{
				{
					Mutation: kyvernov1.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources:   kyvernov1.MatchResources{},
					ExcludeResources: kyvernov1.MatchResources{},
				},
				{
					Mutation: kyvernov1.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources:   kyvernov1.MatchResources{},
					ExcludeResources: kyvernov1.MatchResources{},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
				webhookUpdate: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := computeOperationsForMutatingWebhookConf(testCase.rules, make(map[string]bool))
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}

func TestComputeOperationsForValidatingWebhookConf(t *testing.T) {
	testCases := []struct {
		name           string
		rules          []kyvernov1.Rule
		expectedResult map[string]bool
	}{
		{
			name: "Test Case 1",
			rules: []kyvernov1.Rule{
				{
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Operations: []v1.AdmissionOperation{"CREATE"},
						},
					},
				},
				{
					ExcludeResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Operations: []v1.AdmissionOperation{"DELETE"},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				"CREATE": true,
				"DELETE": true,
			},
		},
		{
			name: "Test Case 2",
			rules: []kyvernov1.Rule{
				{
					MatchResources:   kyvernov1.MatchResources{},
					ExcludeResources: kyvernov1.MatchResources{},
				},
				{
					MatchResources:   kyvernov1.MatchResources{},
					ExcludeResources: kyvernov1.MatchResources{},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate:  true,
				webhookUpdate:  true,
				webhookConnect: true,
				webhookDelete:  true,
			},
		},
		{
			name: "Test Case 3",
			rules: []kyvernov1.Rule{
				{
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Operations: []v1.AdmissionOperation{"CREATE", "UPDATE"},
						},
					},
					ExcludeResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Operations: []v1.AdmissionOperation{"DELETE"},
						},
					},
				},
			},
			expectedResult: map[string]bool{
				webhookCreate: true,
				webhookUpdate: true,
				"DELETE":      true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := computeOperationsForValidatingWebhookConf(testCase.rules, make(map[string]bool))
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}
