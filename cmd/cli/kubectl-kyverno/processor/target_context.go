package processor

import (
	"github.com/kyverno/kyverno/pkg/cel/libs"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// targetResourceContext returns the context the mutate-existing target pass evaluates with, in which CEL
// resource.get() and resource.list() see the supplied target resources. Without a cluster the processor
// already works on a FakeContextProvider and the targets are registered in it; as that provider ignores
// label selectors, lists go through labelFilterContext. With a cluster the targets are laid over the live
// context: they win over live objects with the same identity, and everything else still comes from the
// cluster, so a supplied target that does not exist in the cluster can still be selected.
func targetResourceContext(contextProvider libs.Context, targets []*unstructured.Unstructured, restMapper meta.RESTMapper) libs.Context {
	fake, isFake := contextProvider.(*libs.FakeContextProvider)
	overlay := &targetOverlayContext{Context: contextProvider, targets: map[schema.GroupVersionResource][]*unstructured.Unstructured{}}
	for _, target := range targets {
		gvk := target.GroupVersionKind()
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			continue
		}
		if isFake {
			_ = fake.AddResource(mapping.Resource, target)
			continue
		}
		overlay.targets[mapping.Resource] = append(overlay.targets[mapping.Resource], target)
	}
	if isFake {
		return &labelFilterContext{Context: contextProvider}
	}
	return overlay
}

// labelFilterContext applies the label selector of resource lists, which the FakeContextProvider ignores.
type labelFilterContext struct {
	libs.Context
}

// ListResources returns the listed objects that carry every label of the selector.
func (c *labelFilterContext) ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	list, err := c.Context.ListResources(apiVersion, resource, namespace, labels)
	if err != nil || len(labels) == 0 {
		return list, err
	}
	out := &unstructured.UnstructuredList{Object: list.Object}
	for _, item := range list.Items {
		if matchesLabels(item.GetLabels(), labels) {
			out.Items = append(out.Items, item)
		}
	}
	return out, nil
}

// Clone keeps the label filtering on the cloned context.
func (c *labelFilterContext) Clone() libs.Context {
	return &labelFilterContext{Context: c.Context.Clone()}
}

// targetOverlayContext lays the supplied target resources over a live context for resource lookups.
type targetOverlayContext struct {
	libs.Context
	targets map[schema.GroupVersionResource][]*unstructured.Unstructured
}

// supplied returns the supplied targets of a resource in namespace, or in every namespace when it is empty.
func (c *targetOverlayContext) supplied(apiVersion, resource, namespace string) []*unstructured.Unstructured {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil
	}
	var out []*unstructured.Unstructured
	for _, target := range c.targets[gv.WithResource(resource)] {
		if namespace == "" || target.GetNamespace() == namespace {
			out = append(out, target)
		}
	}
	return out
}

// GetResource returns the supplied target with this identity, or else the live object.
func (c *targetOverlayContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	for _, target := range c.supplied(apiVersion, resource, namespace) {
		if target.GetNamespace() == namespace && target.GetName() == name {
			return target.DeepCopy(), nil
		}
	}
	return c.Context.GetResource(apiVersion, resource, namespace, name)
}

// ListResources returns the live objects with the supplied targets laid over them. A supplied target always
// replaces the live object with the same namespace and name, even when only the live object matches the
// label selector, so a stale live copy is never listed in its place. A kind the cluster does not serve
// (NotFound) still lists the supplied targets; any other error, such as a rejected selector, is returned.
func (c *targetOverlayContext) ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	supplied := c.supplied(apiVersion, resource, namespace)
	live, err := c.Context.ListResources(apiVersion, resource, namespace, labels)
	if err != nil {
		if len(supplied) == 0 || !kerrors.IsNotFound(err) {
			return nil, err
		}
		live = &unstructured.UnstructuredList{}
	}
	if len(supplied) == 0 {
		return live, nil
	}
	out := &unstructured.UnstructuredList{Object: live.Object}
	replaced := map[string]bool{}
	for _, item := range supplied {
		replaced[item.GetNamespace()+"/"+item.GetName()] = true
		if matchesLabels(item.GetLabels(), labels) {
			out.Items = append(out.Items, *item.DeepCopy())
		}
	}
	for _, item := range live.Items {
		if !replaced[item.GetNamespace()+"/"+item.GetName()] {
			out.Items = append(out.Items, item)
		}
	}
	return out, nil
}

// Clone keeps the supplied targets on the cloned context.
func (c *targetOverlayContext) Clone() libs.Context {
	return &targetOverlayContext{Context: c.Context.Clone(), targets: c.targets}
}

// matchesLabels reports whether objectLabels match selector as the API server matches a selector built from
// a map: every key must be present with its value, an empty value included.
func matchesLabels(objectLabels, selector map[string]string) bool {
	return labels.SelectorFromSet(selector).Matches(labels.Set(objectLabels))
}
