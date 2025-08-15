package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestMatchesResourceDescriptionWithEmptyExcludeNames(t *testing.T) {
	resourceJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [{
				"name": "test-container",
				"image": "nginx"
			}]
		}
	}`

	resource, err := kubeutils.BytesToUnstructured([]byte(resourceJSON))
	if err != nil {
		t.Fatalf("Failed to convert resource: %v", err)
	}

	rule := kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Names: []string{}, 
			},
		},
		Validation: &kyvernov1.Validation{
			Message: "test validation rule",
			RawPattern: &apiextv1.JSON{
				Raw: []byte(`{"spec":{"containers":[{"image":"trusted-registry/*"}]}}`),
			},
		},
	}

	err = MatchesResourceDescription(*resource, rule, kyvernov2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE")

	if err != nil {
		t.Errorf("Expected resource to match (not be excluded by empty names array), but got error: %v", err)
	}
}

func TestMatchesResourceDescriptionWithEmptyExcludeNamesVsNil(t *testing.T) {
	resourceJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [{
				"name": "test-container",
				"image": "nginx"
			}]
		}
	}`

	resource, err := kubeutils.BytesToUnstructured([]byte(resourceJSON))
	if err != nil {
		t.Fatalf("Failed to convert resource: %v", err)
	}

	ruleWithNilExclusion := kyvernov1.Rule{
		Name: "test-rule-nil",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: nil,
	}

	ruleWithEmptyNames := kyvernov1.Rule{
		Name: "test-rule-empty",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Names: []string{}, // Empty array
			},
		},
	}

	errNil := MatchesResourceDescription(*resource, ruleWithNilExclusion, kyvernov2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE")
	errEmpty := MatchesResourceDescription(*resource, ruleWithEmptyNames, kyvernov2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE")

	if errNil != nil {
		t.Errorf("Expected resource to match with nil exclusion, but got error: %v", errNil)
	}
	if errEmpty != nil {
		t.Errorf("Expected resource to match with empty names exclusion, but got error: %v", errEmpty)
	}

	if (errNil == nil) != (errEmpty == nil) {
		t.Errorf("Expected nil exclusion and empty names exclusion to behave the same, but got different results: nil=%v, empty=%v", errNil, errEmpty)
	}
}

func TestMatchesResourceDescriptionWithSpecificExclusion(t *testing.T) {
	resourceJSON := `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "default"
		},
		"spec": {
			"containers": [{
				"name": "test-container",
				"image": "nginx"
			}]
		}
	}`

	resource, err := kubeutils.BytesToUnstructured([]byte(resourceJSON))
	if err != nil {
		t.Fatalf("Failed to convert resource: %v", err)
	}

	ruleWithNonMatchingExclusion := kyvernov1.Rule{
		Name: "test-rule-non-matching",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Names: []string{"some-other-pod"}, 
			},
		},
	}

	ruleWithMatchingExclusion := kyvernov1.Rule{
		Name: "test-rule-matching",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Names: []string{"test-pod"},
			},
		},
	}

	errNonMatching := MatchesResourceDescription(*resource, ruleWithNonMatchingExclusion, kyvernov2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE")
	if errNonMatching != nil {
		t.Errorf("Expected resource to match (not be excluded by non-matching name), but got error: %v", errNonMatching)
	}

	errMatching := MatchesResourceDescription(*resource, ruleWithMatchingExclusion, kyvernov2.RequestInfo{}, nil, "", resource.GroupVersionKind(), "", "CREATE")
	if errMatching == nil {
		t.Errorf("Expected resource to be excluded by matching name, but got no error")
	}
}
