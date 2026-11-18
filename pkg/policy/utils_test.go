package policy

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_resourceMatches_Selector(t *testing.T) {
	tests := []struct {
		name     string
		selector *metav1.LabelSelector
		labels   map[string]interface{}
		want     bool
	}{
		{
			name: "exact matchLabels match",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
			labels: map[string]interface{}{"env": "prod"},
			want:   true,
		},
		{
			name: "exact matchLabels mismatch",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"env": "prod"},
			},
			labels: map[string]interface{}{"env": "dev"},
			want:   false,
		},
		{
			name: "wildcard value in matchLabels matches",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "*s3-read*"},
			},
			labels: map[string]interface{}{"app": "my-s3-read-policy"},
			want:   true,
		},
		{
			name: "wildcard value in matchLabels does not match",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "*s3-read*"},
			},
			labels: map[string]interface{}{"app": "my-ec2-policy"},
			want:   false,
		},
		{
			name: "wildcard key and value in matchLabels matches",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"dev/*": "?*"},
			},
			labels: map[string]interface{}{"dev/aws-role": "anything"},
			want:   true,
		},
		{
			name: "wildcard key in matchLabels has no matching resource label",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"dev/*": "?*"},
			},
			labels: map[string]interface{}{"other": "value"},
			want:   false,
		},
		{
			name: "matchExpressions Exists is honored",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpExists},
				},
			},
			labels: map[string]interface{}{"env": "prod"},
			want:   true,
		},
		{
			name: "matchExpressions Exists fails when label absent",
			selector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "env", Operator: metav1.LabelSelectorOpExists},
				},
			},
			labels: map[string]interface{}{},
			want:   false,
		},
		{
			name:     "nil selector always matches",
			selector: nil,
			labels:   map[string]interface{}{},
			want:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := kyverno.ResourceDescription{
				Kinds:    []string{"Namespace"},
				Selector: tt.selector,
			}
			res := unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Namespace",
					"metadata": map[string]interface{}{
						"name":   "my-ns",
						"labels": tt.labels,
					},
				},
			}
			if got := resourceMatches(match, res, false); got != tt.want {
				t.Errorf("resourceMatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_serverSideSelector(t *testing.T) {
	exists := metav1.LabelSelectorRequirement{Key: "tier", Operator: metav1.LabelSelectorOpExists}
	tests := []struct {
		name     string
		selector *metav1.LabelSelector
		want     *metav1.LabelSelector
	}{
		{
			name:     "nil selector",
			selector: nil,
			want:     nil,
		},
		{
			name: "selector without wildcards is kept",
			selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
			want: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
		},
		{
			name: "only wildcarded matchLabels are dropped",
			selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"env": "prod", "app": "*s3-read*", "dev/*": "x"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
			want: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
		},
		{
			name: "expressions are kept when all matchLabels are wildcarded",
			selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"app": "*s3-read*"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
			want: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
		},
		{
			name: "nothing left to filter on",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "*s3-read*"},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, serverSideSelector(tt.selector))
		})
	}
}

// recordingListClient records the selector passed to ListResource and returns a fixed list.
type recordingListClient struct {
	dclient.Interface
	items    []unstructured.Unstructured
	selector *metav1.LabelSelector
	calls    int
}

func (c *recordingListClient) ListResource(_ context.Context, _, _, _ string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	c.calls++
	c.selector = lselector
	return &unstructured.UnstructuredList{Items: c.items}, nil
}

func newLabeledNamespace(name string, labels map[string]interface{}) unstructured.Unstructured {
	return unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name":   name,
				"labels": labels,
			},
		},
	}
}

func Test_getResources_WildcardSelector(t *testing.T) {
	exists := metav1.LabelSelectorRequirement{Key: "tier", Operator: metav1.LabelSelectorOpExists}
	tests := []struct {
		name         string
		selector     *metav1.LabelSelector
		wantSelector *metav1.LabelSelector
	}{
		{
			name: "only wildcarded matchLabels",
			selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "*s3-read*"},
			},
			wantSelector: nil,
		},
		{
			name: "wildcarded matchLabels with exact labels and expressions",
			selector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"app": "*s3-read*", "env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
			wantSelector: &metav1.LabelSelector{
				MatchLabels:      map[string]string{"env": "prod"},
				MatchExpressions: []metav1.LabelSelectorRequirement{exists},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &recordingListClient{
				items: []unstructured.Unstructured{
					newLabeledNamespace("matching", map[string]interface{}{"app": "my-s3-read-policy", "env": "prod", "tier": "backend"}),
					newLabeledNamespace("non-matching", map[string]interface{}{"app": "my-ec2-policy", "env": "prod", "tier": "backend"}),
				},
			}
			match := kyverno.ResourceDescription{
				Kinds:    []string{"Namespace"},
				Selector: tt.selector,
			}

			got := getResources(client, "", false, match, logr.Discard())

			assert.Equal(t, 1, client.calls)
			assert.Equal(t, tt.wantSelector, client.selector)
			if assert.Len(t, got, 1) {
				assert.Equal(t, "matching", got[0].GetName())
			}
		})
	}
}

func TestCastPolicy_MutatingPolicy(t *testing.T) {
	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mpol"},
	}
	result := castPolicy(mpol)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AsMutatingPolicy())
	assert.Equal(t, "test-mpol", result.AsMutatingPolicy().GetName())
	assert.IsType(t, engineapi.NewMutatingPolicy(mpol), result)
}

func TestCastPolicy_NamespacedMutatingPolicy(t *testing.T) {
	nmpol := &policiesv1beta1.NamespacedMutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-nmpol", Namespace: "test-ns"},
	}
	result := castPolicy(nmpol)
	assert.NotNil(t, result)
	assert.NotNil(t, result.AsNamespacedMutatingPolicy())
	assert.Equal(t, "test-nmpol", result.AsNamespacedMutatingPolicy().GetName())
	assert.Equal(t, "test-ns", result.AsNamespacedMutatingPolicy().GetNamespace())
	assert.IsType(t, engineapi.NewNamespacedMutatingPolicy(nmpol), result)
}
