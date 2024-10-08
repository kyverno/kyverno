package webhook

import (
	"cmp"
	"reflect"
	"slices"
	"sort"
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestAddOperationsForValidatingWebhookConfMultiplePolicies(t *testing.T) {
	testCases := []struct {
		name           string
		policies       []kyverno.ClusterPolicy
		expectedResult map[string][]admissionregistrationv1.OperationType
	}{
		{
			name: "test-1",
			policies: []kyverno.ClusterPolicy{
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds: []string{"ConfigMap"},
									},
								},
							},
						},
					},
				},
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds:      []string{"ConfigMap"},
										Operations: []kyverno.AdmissionOperation{"DELETE"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"configmaps": {"CREATE", "UPDATE", "DELETE", "CONNECT"},
			},
		}, {
			name: "test-2",
			policies: []kyverno.ClusterPolicy{
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds:      []string{"Role"},
										Operations: []kyverno.AdmissionOperation{"DELETE"},
									},
								},
							},
						},
					},
				},
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds:      []string{"Secret"},
										Operations: []kyverno.AdmissionOperation{"CONNECT"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"roles":   {"DELETE"},
				"secrets": {"CONNECT"},
			},
		},
	}

	var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			for _, p := range test.policies {
				c := controller{
					discoveryClient: dclient.NewFakeDiscoveryClient(
						[]schema.GroupVersionResource{
							{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
						},
					),
				}
				mapResourceToOpnType = c.addOpnForValidatingWebhookConf(p.GetSpec().Rules, mapResourceToOpnType)
			}
			for key, expectedValue := range test.expectedResult {
				slices.SortFunc(expectedValue, func(a, b admissionregistrationv1.OperationType) int {
					return cmp.Compare(a, b)
				})
				value := mapResourceToOpnType[key]
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
				"configmaps": {"CREATE"},
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
					ExcludeResources: &kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []kyverno.AdmissionOperation{"DELETE", "CONNECT", "CREATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"configmaps": {"UPDATE"},
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
				"configmaps": {"CREATE", "UPDATE", "DELETE", "CONNECT"},
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
				"configmaps": {"CREATE", "UPDATE"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			c := controller{
				discoveryClient: dclient.NewFakeDiscoveryClient([]schema.GroupVersionResource{}),
			}
			result = c.addOpnForValidatingWebhookConf(testCase.rules, mapResourceToOpnType)

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
					Mutation: &kyverno.Mutation{
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
				"configmaps": {"CREATE"},
			},
		},
		{
			name: "Test Case 2",
			rules: []kyverno.Rule{
				{
					Mutation: &kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Secret"},
						},
					},
					ExcludeResources: &kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Operations: []kyverno.AdmissionOperation{"UPDATE"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"secrets": {"CREATE"},
			},
		},
		{
			name: "Test Case 3",
			rules: []kyverno.Rule{
				{
					Mutation: &kyverno.Mutation{
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
					Mutation: &kyverno.Mutation{
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
				"secrets": {"CREATE", "UPDATE"},
			},
		},
		{
			name: "Test Case 4",
			rules: []kyverno.Rule{
				{
					Mutation: &kyverno.Mutation{
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
					Mutation: &kyverno.Mutation{
						PatchesJSON6902: "add",
					},
					MatchResources: kyverno.MatchResources{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Secret"},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"secrets": {"CREATE", "UPDATE"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var result map[string][]admissionregistrationv1.OperationType
			var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
			c := controller{
				discoveryClient: dclient.NewFakeDiscoveryClient(
					[]schema.GroupVersionResource{},
				),
			}
			result = c.addOpnForMutatingWebhookConf(testCase.rules, mapResourceToOpnType)

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

func TestAddOperationsForMutatingtingWebhookConfMultiplePolicies(t *testing.T) {
	testCases := []struct {
		name           string
		policies       []kyverno.ClusterPolicy
		expectedResult map[string][]admissionregistrationv1.OperationType
	}{
		{
			name: "test-1",
			policies: []kyverno.ClusterPolicy{
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								Mutation: &kyverno.Mutation{
									RawPatchStrategicMerge: &apiextensionsv1.JSON{Raw: []byte(`"nodeSelector": {<"public-ip-type": "elastic"}, +"priorityClassName": "elastic-ip-required"`)},
								},
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds: []string{"Pod"},
									},
								},
							},
						},
					},
				},
				{
					Spec: kyverno.Spec{
						Rules: []kyverno.Rule{
							{
								Generation: &kyverno.Generation{},
								MatchResources: kyverno.MatchResources{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds: []string{"Deployment", "StatefulSet", "DaemonSet", "Job"},
									},
								},
							},
						},
					},
				},
			},
			expectedResult: map[string][]admissionregistrationv1.OperationType{
				"pods": {"CREATE", "UPDATE"},
			},
		},
	}

	var mapResourceToOpnType map[string][]admissionregistrationv1.OperationType
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			for _, p := range test.policies {
				c := controller{
					discoveryClient: dclient.NewFakeDiscoveryClient(
						[]schema.GroupVersionResource{
							{Version: "v1", Resource: "pods"},
						},
					),
				}
				mapResourceToOpnType = c.addOpnForMutatingWebhookConf(p.GetSpec().Rules, mapResourceToOpnType)
			}
			if !compareMaps(mapResourceToOpnType, test.expectedResult) {
				t.Errorf("Expected %v, but got %v", test.expectedResult, mapResourceToOpnType)
			}
		})
	}
}

func compareMaps(a, b map[string][]admissionregistrationv1.OperationType) bool {
	if len(a) != len(b) {
		return false
	}

	for key, aValue := range a {
		bValue, ok := b[key]
		if !ok {
			return false
		}

		sort.Slice(aValue, func(i, j int) bool {
			return cmp.Compare(aValue[i], aValue[j]) < 0
		})
		sort.Slice(bValue, func(i, j int) bool {
			return cmp.Compare(bValue[i], bValue[j]) < 0
		})

		if !reflect.DeepEqual(aValue, bValue) {
			return false
		}
	}

	return true
}
