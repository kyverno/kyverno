package common

import (
	"context"
	"strings"
	"testing"

	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type groupCollisionClient struct {
	dclient.Interface
}

// ListResource returns a same-named parent resource for each requested API group.
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

// TestListResourcesPreservesAPIGroups verifies that resource keys retain distinct API groups.
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

			resources, err := fetcher.listResources(t.Context(), info)
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

// GetResource marks the returned object with the requested subresource for assertions.
func (groupCollisionClient) GetResource(
	ctx context.Context,
	apiVersion string,
	kind string,
	namespace string,
	name string,
	subresources ...string,
) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
	resource.SetAnnotations(map[string]string{
		"test.kyverno.io/subresource": strings.Join(subresources, "/"),
		"test.kyverno.io/parent-kind": kind,
	})
	return resource, nil
}

// TestListSubresourcesPreservesAPIGroups verifies that subresource keys retain distinct API groups.
func TestListSubresourcesPreservesAPIGroups(t *testing.T) {
	groups := []string{
		"alpha.example.com",
		"beta.example.com",
	}

	info := &resourceTypeInfo{
		subresourceMap: make(
			map[subresourceKey]v1alpha1.Subresource,
		),
	}

	for _, group := range groups {
		gvk := schema.GroupVersionKind{
			Group:   group,
			Version: "v1",
			Kind:    "Widget",
		}

		info.subresourceMap[subresourceKey{GroupVersionKind: gvk, name: "status"}] = v1alpha1.Subresource{
			ParentResource: metav1.APIResource{
				Group:      group,
				Version:    "v1",
				Kind:       "Widget",
				Name:       "widgets",
				Namespaced: true,
			},
			Subresource: metav1.APIResource{
				Group:      group,
				Version:    "v1",
				Kind:       "Widget",
				Name:       "widgets/status",
				Namespaced: true,
			},
		}
	}

	fetcher := &ResourceFetcher{
		Client:    groupCollisionClient{},
		Namespace: "default",
	}

	resources, err := fetcher.listResources(t.Context(), info)
	if err != nil {
		t.Fatal(err)
	}

	if len(resources) != len(groups) {
		t.Fatalf(
			"got %d subresources, want %d",
			len(resources),
			len(groups),
		)
	}

	seenGroups := make(map[string]bool)
	for _, resource := range resources {
		seenGroups[resource.GroupVersionKind().Group] = true
	}

	for _, group := range groups {
		if !seenGroups[group] {
			t.Errorf("missing subresource from API group %q", group)
		}
	}
}

// TestListResourcesPreservesParentAndSubresource verifies that matching GVKs and names do not discard either input.
func TestListResourcesPreservesParentAndSubresource(t *testing.T) {
	for _, subresourceName := range []string{"status", "resize"} {
		t.Run(subresourceName, func(t *testing.T) {
			gvk := schema.GroupVersionKind{
				Group: "alpha.example.com", Version: "v1", Kind: "Widget",
			}
			info := &resourceTypeInfo{
				gvkMap: map[schema.GroupVersionKind]bool{gvk: true},
				subresourceMap: map[subresourceKey]v1alpha1.Subresource{
					{GroupVersionKind: gvk, name: subresourceName}: {
						ParentResource: metav1.APIResource{
							Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
							Name: "widgets", Namespaced: true,
						},
						Subresource: metav1.APIResource{
							Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
							Name: "widgets/" + subresourceName, Namespaced: true,
						},
					},
				},
			}
			fetcher := &ResourceFetcher{
				Client: groupCollisionClient{}, Namespace: "default",
			}
			resources, err := fetcher.listResources(t.Context(), info)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 2 {
				t.Fatalf("got %d resources, want both parent and %s subresource", len(resources), subresourceName)
			}
			seen := make(map[string]int)
			for _, resource := range resources {
				if resource.GroupVersionKind() != gvk || resource.GetNamespace() != "default" || resource.GetName() != "shared" {
					t.Errorf("unexpected resource identity: %s %s/%s", resource.GroupVersionKind(), resource.GetNamespace(), resource.GetName())
				}
				seen[resource.GetAnnotations()["test.kyverno.io/subresource"]]++
			}
			if seen[""] != 1 || seen[subresourceName] != 1 {
				t.Errorf("expected one parent and one %s subresource, got %v", subresourceName, seen)
			}
		})
	}
}
