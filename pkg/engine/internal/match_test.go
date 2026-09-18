package internal

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockConfig struct {
	config.Configuration
	toFilter func(kind schema.GroupVersionKind, subresource, namespace, name string) bool
}

func (m *mockConfig) ToFilter(kind schema.GroupVersionKind, subresource, namespace, name string) bool {
	return m.toFilter(kind, subresource, namespace, name)
}

func Test_checkResourceFilters(t *testing.T) {
	tests := []struct {
		name          string
		resource      unstructured.Unstructured
		filterName    string // the name we expect ToFilter to be called with
		shouldFilter  bool
		expectedMatch bool
	}{
		{
			name: "resource with name",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "test-pod",
						"namespace": "default",
					},
				},
			},
			filterName:    "test-pod",
			shouldFilter:  true,
			expectedMatch: false, // if it filters, it returns false
		},
		{
			name: "resource with generateName",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"generateName": "test-pod-",
						"namespace":    "default",
					},
				},
			},
			filterName:    "test-pod-",
			shouldFilter:  true,
			expectedMatch: false,
		},
		{
			name: "resource with neither",
			resource: unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"namespace": "default",
					},
				},
			},
			filterName:    "",
			shouldFilter:  false,
			expectedMatch: true,
		},
	}

	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	subresource := ""

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &mockConfig{
				toFilter: func(kind schema.GroupVersionKind, sub, namespace, name string) bool {
					if name != tt.filterName {
						t.Errorf("expected ToFilter to be called with name %q, got %q", tt.filterName, name)
					}
					return tt.shouldFilter
				},
			}

			result := checkResourceFilters(cfg, gvk, subresource, tt.resource)
			if result != tt.expectedMatch {
				t.Errorf("expected %v, got %v", tt.expectedMatch, result)
			}
		})
	}
}
