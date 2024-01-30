package webhook

import (
	"reflect"
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
			name: "Test Case 1",
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			result = addOpnForValidatingWebhookConf(testCase.rules, mapResourceToOpnType)

			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
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
			name: "Test Case 1",
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
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			result = addOpnForMutatingWebhookConf(testCase.rules, mapResourceToOpnType)

			if !reflect.DeepEqual(result, testCase.expectedResult) {
				t.Errorf("Expected %v, but got %v", testCase.expectedResult, result)
			}
		})
	}
}
