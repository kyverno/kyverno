package webhook

import (
	"reflect"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	v1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func TestGetMinimumOperations(t *testing.T) {
	// Define a test case with an input map and expected result
	testCases := []struct {
		name           string
		inputMap       map[string]bool
		expectedResult []string
	}{
		{
			name: "Test Case 1",
			inputMap: map[string]bool{
				"CREATE": true,
				"UPDATE": false,
				"DELETE": true,
			},
			expectedResult: []string{"CREATE", "DELETE"},
		},
		{
			name: "Test Case 2",
			inputMap: map[string]bool{
				"CREATE":  false,
				"UPDATE":  false,
				"DELETE":  false,
				"CONNECT": true,
			},
			expectedResult: []string{"CONNECT"},
		},
		// Add more test cases as needed
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := getMinimumOperations(testCase.inputMap)

			// Ensure the lengths of both slices are equal
			if len(result) != len(testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
				return
			}

			// Compare elements while maintaining order
			for i := range result {
				if string(result[i]) != testCase.expectedResult[i] {
					t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
					return
				}
			}
		})
	}
}

func TestComputeOperationsForMutatingWebhookConf(t *testing.T) {
	// Define a test case with input rules and expected result
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
			},
			expectedResult: map[string]bool{
				"CREATE": true,
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
				webhookCreate: true,
				webhookUpdate: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := computeOperationsForMutatingWebhookConf(testCase.rules, make(map[string]bool))

			// Ensure that the result matches the expected result using deep equality
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}

func TestComputeOperationsForValidatingWebhookConf(t *testing.T) {
	// Define a test case with input rules and expected result
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
				// {
				// 	ExcludeResources: kyvernov1.MatchResources{
				// 		ResourceDescription: kyvernov1.ResourceDescription{
				// 			Operations: []v1.AdmissionOperation{"DELETE"},
				// 		},
				// 	},
				// },
			},
			expectedResult: map[string]bool{
				"CREATE": true,
				// "DELETE": true,
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
				webhookCreate: true,
				webhookUpdate: true,
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

			// Ensure that the result matches the expected result using deep equality
			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}
