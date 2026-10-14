package processor

import (
	"github.com/kyverno/kyverno/pkg/cel/libs"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// targetResourceContext returns the context the mutate-existing target pass evaluates with, in which CEL
// resource.get() and resource.list() see the supplied target resources. Without a cluster the processor
// already works on a FakeContextProvider and the targets are registered in it. With a cluster the targets
// are laid over the live context: they win over live objects with the same identity, and everything else
// still comes from the cluster, so a supplied target that does not exist in the cluster can still be selected.
func targetResourceContext(contextProvider libs.Context, targets []*unstructured.Unstructured, restMapper meta.RESTMapper) libs.Context {
	fake, isFake := contextProvider.(*libs.FakeContextProvider)
	if !isFake {
		fake = libs.NewFakeContextProvider()
	}
	for _, target := range targets {
		gvk := target.GroupVersionKind()
		mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			continue
		}
		_ = fake.AddResource(mapping.Resource, target)
	}
	if isFake {
		return contextProvider
	}
	return &targetOverlayContext{Context: contextProvider, targets: fake}
}

// targetOverlayContext lays the supplied target resources over a live context for resource lookups.
type targetOverlayContext struct {
	libs.Context
	targets *libs.FakeContextProvider
}

// GetResource returns the supplied target with this identity, or else the live object.
func (c *targetOverlayContext) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	if object, err := c.targets.GetResource(apiVersion, resource, namespace, name); err == nil {
		return object, nil
	}
	return c.Context.GetResource(apiVersion, resource, namespace, name)
}

// ListResources returns the live objects with every supplied target added, a supplied target replacing a
// live object with the same name. A kind that is missing from the cluster still lists the supplied targets.
func (c *targetOverlayContext) ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
	supplied, suppliedErr := c.targets.ListResources(apiVersion, resource, namespace, labels)
	live, err := c.Context.ListResources(apiVersion, resource, namespace, labels)
	if err != nil {
		if suppliedErr != nil || (!kerrors.IsNotFound(err) && !kerrors.IsBadRequest(err)) {
			return nil, err
		}
		live = &unstructured.UnstructuredList{}
	}
	if suppliedErr != nil {
		return live, nil
	}
	out := &unstructured.UnstructuredList{Object: live.Object}
	replaced := map[string]bool{}
	for _, item := range supplied.Items {
		if matchesLabels(item.GetLabels(), labels) {
			out.Items = append(out.Items, item)
			replaced[item.GetName()] = true
		}
	}
	for _, item := range live.Items {
		if !replaced[item.GetName()] {
			out.Items = append(out.Items, item)
		}
	}
	return out, nil
}

// Clone keeps the supplied targets on the cloned context.
func (c *targetOverlayContext) Clone() libs.Context {
	return &targetOverlayContext{Context: c.Context.Clone(), targets: c.targets}
}

// matchesLabels reports whether labels carries every key and value in selector.
func matchesLabels(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}
