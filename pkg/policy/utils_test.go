package policy

import (
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_resourceMatches(t *testing.T) {
	tests := []struct {
		name               string
		match              kyverno.ResourceDescription
		res                unstructured.Unstructured
		isNamespacedPolicy bool
		want               bool
	}{
		{
			name: "Matching resource based on its name",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Names: []string{"my-pod", "test-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "my-pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Non-matching resource based on its name",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Names: []string{"test-pod", "test-pod-1"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "my-pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Matching resource with a wildcard name",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Name:  "my-*",
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "my-pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Non-matching resource with a wildcard name",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Name:  "my-*",
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "test-pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Matching resource with multiple wildcard names",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Names: []string{"my-*", "test-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "my-pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Non-matching resource with multiple wildcard names",
			match: kyverno.ResourceDescription{
				Kinds: []string{"Pod"},
				Names: []string{"my-*", "test-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "pod",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Matching resource based on its namespace",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-ns",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Non-matching resource based on its namespace",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "default",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Matching resource with a namespaced policy",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "default",
					},
				},
			},
			isNamespacedPolicy: true,
			want:               true,
		},
		{
			name: "Matching resource based on its name and namespace",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
				Names:      []string{"my-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-ns",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Non-matching resource based on its name and namespace",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
				Names:      []string{"my-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "default",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Non-matching resource based on its name and namespace",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns"},
				Kinds:      []string{"Pod"},
				Names:      []string{"test-pod-1", "test-pod-2"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-ns",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               false,
		},
		{
			name: "Matching resource based on its name and namespace where the namespace is a wildcard",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns", "test-ns1?"},
				Kinds:      []string{"Pod"},
				Names:      []string{"my-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-ns1a",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
		{
			name: "Matching resource based on its name and namespace where the namespace has multiple wildcards",
			match: kyverno.ResourceDescription{
				Namespaces: []string{"test-ns", "te*t-n*"},
				Kinds:      []string{"Pod"},
				Names:      []string{"my-pod"},
			},
			res: unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-ns1",
					},
				},
			},
			isNamespacedPolicy: false,
			want:               true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceMatches(tt.match, tt.res, tt.isNamespacedPolicy); got != tt.want {
				t.Errorf("resourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_resourceMatchesWithWildcards(t *testing.T) {
	tests := []struct {
		name               string
		namespaces         []string
		isNamespacedPolicy bool
		want               bool
	}{
		{
			name:       "in a cluster policy, the desired namespaces have no wildcards, and the resource namespace is an exact match",
			namespaces: []string{"default", "test-namespace"},
			want:       true,
		},
		{
			name:       "in a cluster policy, the desired namespaces have no wildcards, but the resource namespace is NOT an exact match",
			namespaces: []string{"default", "test-namespace2"},
			want:       false,
		},
		{
			name:       "in a cluster policy, the desired namespaces have wildcards, and the resource namespace matches at least one",
			namespaces: []string{"default", "test-*"},
			want:       true,
		},
		{
			name:       "in a cluster policy, the desired namespaces have wildcards, but the resource namespace matches NOT even one",
			namespaces: []string{"default", "tes-*"},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := kyverno.ResourceDescription{
				Namespaces: tt.namespaces,
				Kinds:      []string{"Pod"},
				Names:      []string{"my-pod"},
			}
			res := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name":      "my-pod",
						"namespace": "test-namespace",
					},
				},
			}
			if got := resourceMatches(match, res, tt.isNamespacedPolicy); got != tt.want {
				t.Errorf("resourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
