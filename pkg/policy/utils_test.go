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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resourceMatches(tt.match, tt.res, tt.isNamespacedPolicy); got != tt.want {
				t.Errorf("resourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}
