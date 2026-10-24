package common

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type groupCollisionClient struct {
	dclient.Interface
}

func (groupCollisionClient) ListResource(
	ctx context.Context,
	apiVersion string,
	kind string,
	namespace string,
	selector *metav1.LabelSelector,
) (*unstructured.UnstructuredList, error) {
	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      "shared",
				"namespace": namespace,
			},
		},
	}

	return &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{resource},
	}, nil
}

func TestListResourcesPreservesAPIGroups(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
	}{
		{
			name:   "single API group",
			groups: []string{"alpha.example.com"},
		},
		{
			name: "different API groups with identical resource names",
			groups: []string{
				"alpha.example.com",
				"beta.example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &resourceTypeInfo{
				gvkMap: make(map[schema.GroupVersionKind]bool),
			}

			for _, group := range tt.groups {
				info.gvkMap[schema.GroupVersionKind{
					Group:   group,
					Version: "v1",
					Kind:    "Widget",
				}] = true
			}

			fetcher := &ResourceFetcher{
				Client:    groupCollisionClient{},
				Namespace: "default",
			}

			resources, err := fetcher.listResources(info)
			if err != nil {
				t.Fatal(err)
			}

			if len(resources) != len(tt.groups) {
				t.Fatalf(
					"got %d resources, want %d",
					len(resources),
					len(tt.groups),
				)
			}

			seenGroups := make(map[string]bool)
			for _, resource := range resources {
				seenGroups[resource.GroupVersionKind().Group] = true
			}

			for _, group := range tt.groups {
				if !seenGroups[group] {
					t.Errorf("missing resource from API group %q", group)
				}
			}
		})
	}
}
