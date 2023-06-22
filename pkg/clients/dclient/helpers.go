package dclient

import (
	"context"

	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Resource struct {
	Group        string
	Version      string
	Resource     string
	SubResource  string
	Unstructured unstructured.Unstructured
}

func GetResources(ctx context.Context, c Interface, group, version, kind, subresource, namespace, name string) ([]Resource, error) {
	var resources []Resource
	gvrss, err := c.Discovery().FindResources(group, version, kind, subresource)
	if err != nil {
		return nil, err
	}
	for gvrs := range gvrss {
		dyn := c.GetDynamicInterface().Resource(gvrs.GroupVersionResource())
		var sub []string
		if gvrs.SubResource != "" {
			sub = []string{gvrs.SubResource}
		}
		// we can use `GET` directly
		if namespace != "" && name != "" && !wildcard.ContainsWildcard(namespace) && !wildcard.ContainsWildcard(name) {
			var obj *unstructured.Unstructured
			var err error
			obj, err = dyn.Namespace(namespace).Get(ctx, name, metav1.GetOptions{}, sub...)
			if err != nil {
				return nil, err
			}
			resources = append(resources, Resource{
				Group:        gvrs.Group,
				Version:      gvrs.Version,
				Resource:     gvrs.Resource,
				SubResource:  gvrs.SubResource,
				Unstructured: *obj,
			})
		} else {
			// we can use `LIST`
			if gvrs.SubResource == "" {
				list, err := dyn.List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				for _, obj := range list.Items {
					if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
						resources = append(resources, Resource{
							Group:        gvrs.Group,
							Version:      gvrs.Version,
							Resource:     gvrs.Resource,
							SubResource:  gvrs.SubResource,
							Unstructured: obj,
						})
					}
				}
			} else {
				// we need to use `LIST` / `GET`
				list, err := dyn.List(ctx, metav1.ListOptions{})
				if err != nil {
					return nil, err
				}
				var parentObjects []unstructured.Unstructured
				for _, obj := range list.Items {
					if match(namespace, name, obj.GetNamespace(), obj.GetName()) {
						parentObjects = append(parentObjects, obj)
					}
				}
				for _, parentObject := range parentObjects {
					var obj *unstructured.Unstructured
					var err error
					if parentObject.GetNamespace() == "" {
						obj, err = dyn.Get(ctx, name, metav1.GetOptions{}, sub...)
					} else {
						obj, err = dyn.Namespace(parentObject.GetNamespace()).Get(ctx, name, metav1.GetOptions{}, sub...)
					}
					if err != nil {
						return nil, err
					}
					resources = append(resources, Resource{
						Group:        gvrs.Group,
						Version:      gvrs.Version,
						Resource:     gvrs.Resource,
						SubResource:  gvrs.SubResource,
						Unstructured: *obj,
					})
				}
			}
		}
	}
	return resources, nil
}

func match(namespacePattern, namePattern, namespace, name string) bool {
	if namespacePattern == "" && namePattern == "" {
		return true
	} else if namespacePattern == "" {
		if wildcard.Match(namePattern, name) {
			return true
		}
	} else if wildcard.Match(namespacePattern, namespace) {
		if namePattern == "" || wildcard.Match(namePattern, name) {
			return true
		}
	}
	return false
}
