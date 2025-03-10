package policy

import (
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FakeContextProvider struct {
	resources map[string]map[string]map[string]*unstructured.Unstructured
}

func NewFakeContextProvider() *FakeContextProvider {
	return &FakeContextProvider{
		resources: map[string]map[string]map[string]*unstructured.Unstructured{},
	}
}

func (cp *FakeContextProvider) AddResource(gvr schema.GroupVersionResource, obj runtime.Object) error {
	object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return err
	}
	resource := &unstructured.Unstructured{Object: object}
	resources := cp.resources[gvr.String()]
	if resources == nil {
		resources = map[string]map[string]*unstructured.Unstructured{}
		cp.resources[gvr.String()] = resources
	}
	namespace := resources[resource.GetNamespace()]
	if namespace == nil {
		namespace = map[string]*unstructured.Unstructured{}
		resources[resource.GetNamespace()] = namespace
	}
	namespace[resource.GetName()] = resource
	return nil
}

func (cp *FakeContextProvider) GetConfigMap(ns, n string) (unstructured.Unstructured, error) {
	cm, err := cp.GetResource("v1", "configmaps", ns, n)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	return *cm, nil
}

func (cp *FakeContextProvider) GetGlobalReference(string, string) (any, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) GetImageData(string) (*imagedataloader.ImageData, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) ParseImageReference(string) (imagedataloader.ImageReference, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	gvr := gv.WithResource(resource)
	resources := cp.resources[gvr.String()]
	if resources == nil {
		return nil, kerrors.NewNotFound(gvr.GroupResource(), name)
	}
	namespaced := resources[namespace]
	if namespaced == nil {
		return nil, kerrors.NewNotFound(gvr.GroupResource(), name)
	}
	resourced := namespaced[name]
	if resourced == nil {
		return nil, kerrors.NewNotFound(gvr.GroupResource(), name)
	}
	return resourced, nil
}
