package libs

import (
	"fmt"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FakeContextProvider struct {
	resources          map[string]map[string]map[string]*unstructured.Unstructured
	generatedResources []*unstructured.Unstructured
	policyName         string
	triggerName        string
	triggerNamespace   string
	triggerAPIVersion  string
	triggerGroup       string
	triggerKind        string
	triggerUID         string
	restoreCache       bool
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

func (cp *FakeContextProvider) GetGlobalReference(string, string) (any, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) GetImageData(string) (map[string]any, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	gvr := gv.WithResource(resource)
	resources := cp.resources[gvr.String()]
	if resources == nil {
		return nil, kerrors.NewBadRequest(fmt.Sprintf("%s resource not found", gvr.GroupResource()))
	}
	var out unstructured.UnstructuredList
	for _, obj := range resources[namespace] {
		out.Items = append(out.Items, *obj)
	}
	return &out, nil
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

func (cp *FakeContextProvider) PostResource(string, string, string, map[string]any) (*unstructured.Unstructured, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) GenerateResources(namespace string, dataList []map[string]any) error {
	for _, data := range dataList {
		resource := &unstructured.Unstructured{Object: data}
		resource.SetNamespace(namespace)
		if resource.IsList() {
			resourceList, err := resource.ToList()
			if err != nil {
				return err
			}
			for i := range resourceList.Items {
				item := &resourceList.Items[i]
				item.SetNamespace(namespace)
				cp.generatedResources = append(cp.generatedResources, item)
			}
		} else {
			cp.generatedResources = append(cp.generatedResources, resource)
		}
	}
	return nil
}

func (cp *FakeContextProvider) GetGeneratedResources() []*unstructured.Unstructured {
	return cp.generatedResources
}

func (cp *FakeContextProvider) ClearGeneratedResources() {
	cp.generatedResources = make([]*unstructured.Unstructured, 0)
}

func (cp *FakeContextProvider) SetGenerateContext(polName, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	cp.policyName = polName
	cp.triggerName = triggerName
	cp.triggerNamespace = triggerNamespace
	cp.triggerAPIVersion = triggerAPIVersion
	cp.triggerGroup = triggerGroup
	cp.triggerKind = triggerKind
	cp.triggerUID = triggerUID
	cp.restoreCache = restoreCache
}
