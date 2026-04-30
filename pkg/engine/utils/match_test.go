package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestCheckNameSpace(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		resource   unstructured.Unstructured
		expected   bool
	}{
		{
			name:       "exact namespace match",
			namespaces: []string{"default"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expected: true,
		},
		{
			name:       "namespace not in list",
			namespaces: []string{"production", "staging"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expected: false,
		},
		{
			name:       "wildcard namespace match",
			namespaces: []string{"prod-*"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "prod-us-east",
					},
				},
			},
			expected: true,
		},
		{
			name:       "namespace resource uses name instead of namespace",
			namespaces: []string{"kube-system"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Namespace",
					"metadata": map[string]interface{}{
						"name": "kube-system",
					},
				},
			},
			expected: true,
		},
		{
			name:       "namespace resource name not in list",
			namespaces: []string{"default"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Namespace",
					"metadata": map[string]interface{}{
						"name": "production",
					},
				},
			},
			expected: false,
		},
		{
			name:       "empty namespace list",
			namespaces: []string{},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			expected: false,
		},
		{
			name:       "wildcard match all namespaces",
			namespaces: []string{"*"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "any-namespace",
					},
				},
			},
			expected: true,
		},
		{
			name:       "multiple namespace patterns with one match",
			namespaces: []string{"dev-*", "test-*", "prod-*"},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": "test-environment",
					},
				},
			},
			expected: true,
		},
		{
			name:       "cluster scoped resource with empty namespace",
			namespaces: []string{""},
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "ClusterRole",
					"metadata": map[string]interface{}{
						"name": "admin",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkNameSpace(tt.namespaces, tt.resource)
			if result != tt.expected {
				t.Errorf("checkNameSpace() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckNameSpace_WildcardPatterns(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		namespace   string
		shouldMatch bool
	}{
		{"prefix wildcard", "kube-*", "kube-system", true},
		{"suffix wildcard", "*-system", "kube-system", true},
		{"middle wildcard", "kube-*-system", "kube-test-system", true},
		{"no match", "prod-*", "staging-app", false},
		{"exact match", "default", "default", true},
		{"single char mismatch", "defaultx", "default", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"kind": "Pod",
					"metadata": map[string]interface{}{
						"namespace": tt.namespace,
					},
				},
			}
			result := checkNameSpace([]string{tt.pattern}, resource)
			if result != tt.shouldMatch {
				t.Errorf("checkNameSpace([%q], %q) = %v, want %v",
					tt.pattern, tt.namespace, result, tt.shouldMatch)
			}
		})
	}
}

func TestMatchesResourceDescription_EmptyResource(t *testing.T) {
	resource := unstructured.Unstructured{}

	err := MatchesResourceDescription(
		resource,
		kyvernov1.Rule{Name: "test-rule"},
		kyvernov2.RequestInfo{},
		nil,
		"",
		schema.GroupVersionKind{},
		"",
		"",
	)

	if err == nil {
		t.Error("Expected error for empty resource")
	}
	if err.Error() != "resource is empty" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestMatchesResourceDescription_NamespaceMismatch(t *testing.T) {
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name":      "test-pod",
				"namespace": "production",
			},
		},
	}

	err := MatchesResourceDescription(
		resource,
		kyvernov1.Rule{Name: "test-rule"},
		kyvernov2.RequestInfo{},
		nil,
		"development", // policy namespace doesn't match resource namespace
		schema.GroupVersionKind{Version: "v1", Kind: "Pod"},
		"",
		"CREATE",
	)

	if err == nil {
		t.Error("Expected error for namespace mismatch")
	}
}

// TestExcludeAnyNamespace_KubeSystem verifies that exclude.any.resources.namespaces
// correctly excludes all resources in kube-system (fixes #15646).
func TestExcludeAnyNamespace_KubeSystem(t *testing.T) {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Pod"}

	rule := kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Namespaces: []string{"kube-system"},
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		namespace   string
		wantExclude bool // true means MatchesResourceDescription should return an error (excluded)
	}{
		{
			name:        "pod in kube-system must be excluded",
			namespace:   "kube-system",
			wantExclude: true,
		},
		{
			name:        "pod in default must be evaluated",
			namespace:   "default",
			wantExclude: false,
		},
		{
			name:        "pod in kube-public must be evaluated",
			namespace:   "kube-public",
			wantExclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": tt.namespace,
					},
				},
			}
			err := MatchesResourceDescription(resource, rule, kyvernov2.RequestInfo{}, nil, "", gvk, "", "CREATE")
			if tt.wantExclude && err == nil {
				t.Errorf("expected pod in %q to be excluded (err != nil), but got nil", tt.namespace)
			}
			if !tt.wantExclude && err != nil {
				t.Errorf("expected pod in %q to be evaluated (err == nil), but got: %v", tt.namespace, err)
			}
		})
	}
}

// TestExcludeAnyNamespace_NamesAndNamespaces verifies that when both names and namespaces
// are specified in the same exclude.any filter, AND logic is applied correctly.
func TestExcludeAnyNamespace_NamesAndNamespaces(t *testing.T) {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Pod"}

	rule := kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Names:      []string{"excluded-pod"},
						Namespaces: []string{"kube-system"},
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		podName     string
		namespace   string
		wantExclude bool
	}{
		{
			name:        "matching name AND namespace must be excluded",
			podName:     "excluded-pod",
			namespace:   "kube-system",
			wantExclude: true,
		},
		{
			name:        "matching name but wrong namespace must be evaluated",
			podName:     "excluded-pod",
			namespace:   "default",
			wantExclude: false,
		},
		{
			name:        "matching namespace but wrong name must be evaluated",
			podName:     "other-pod",
			namespace:   "kube-system",
			wantExclude: false,
		},
		{
			name:        "neither name nor namespace match must be evaluated",
			podName:     "other-pod",
			namespace:   "default",
			wantExclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      tt.podName,
						"namespace": tt.namespace,
					},
				},
			}
			err := MatchesResourceDescription(resource, rule, kyvernov2.RequestInfo{}, nil, "", gvk, "", "CREATE")
			if tt.wantExclude && err == nil {
				t.Errorf("expected pod %q in %q to be excluded, but got nil", tt.podName, tt.namespace)
			}
			if !tt.wantExclude && err != nil {
				t.Errorf("expected pod %q in %q to be evaluated, but got: %v", tt.podName, tt.namespace, err)
			}
		})
	}
}

// TestExcludeAnyNamespace_MultipleFilters verifies that exclude.any with multiple
// filters correctly excludes a resource if ANY filter matches.
func TestExcludeAnyNamespace_MultipleFilters(t *testing.T) {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Pod"}

	rule := kyvernov1.Rule{
		Name: "test-rule",
		MatchResources: kyvernov1.MatchResources{
			ResourceDescription: kyvernov1.ResourceDescription{
				Kinds: []string{"Pod"},
			},
		},
		ExcludeResources: &kyvernov1.MatchResources{
			Any: kyvernov1.ResourceFilters{
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Namespaces: []string{"kube-system"},
					},
				},
				{
					ResourceDescription: kyvernov1.ResourceDescription{
						Names: []string{"special-pod"},
					},
				},
			},
		},
	}

	tests := []struct {
		name        string
		podName     string
		namespace   string
		wantExclude bool
	}{
		{
			name:        "pod in kube-system excluded by namespace filter",
			podName:     "any-pod",
			namespace:   "kube-system",
			wantExclude: true,
		},
		{
			name:        "special-pod in default excluded by name filter",
			podName:     "special-pod",
			namespace:   "default",
			wantExclude: true,
		},
		{
			name:        "special-pod in kube-system excluded by both filters",
			podName:     "special-pod",
			namespace:   "kube-system",
			wantExclude: true,
		},
		{
			name:        "regular pod in default must be evaluated",
			podName:     "regular-pod",
			namespace:   "default",
			wantExclude: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      tt.podName,
						"namespace": tt.namespace,
					},
				},
			}
			err := MatchesResourceDescription(resource, rule, kyvernov2.RequestInfo{}, nil, "", gvk, "", "CREATE")
			if tt.wantExclude && err == nil {
				t.Errorf("expected pod %q in %q to be excluded, but got nil", tt.podName, tt.namespace)
			}
			if !tt.wantExclude && err != nil {
				t.Errorf("expected pod %q in %q to be evaluated, but got: %v", tt.podName, tt.namespace, err)
			}
		})
	}
}
