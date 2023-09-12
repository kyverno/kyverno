package resource

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ResourceKey struct {
	schema.GroupKind
	Namespace string
	Name      string
}

type ResourceMap = map[ResourceKey]*unstructured.Unstructured

func RemoveDuplicates(resources []*unstructured.Unstructured) (ResourceMap, ResourceMap) {
	duplicates := ResourceMap{}
	uniques := ResourceMap{}
	for _, resource := range resources {
		if resource != nil {
			key := ResourceKey{
				GroupKind: resource.GroupVersionKind().GroupKind(),
				Namespace: resource.GetNamespace(),
				Name:      resource.GetName(),
			}
			if uniques[key] == nil {
				uniques[key] = resource
			} else {
				duplicates[key] = resource
			}
		}
	}
	return uniques, duplicates
}
