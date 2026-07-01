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
	images             map[string]map[string]any
	globalReferences   map[string]any
	httpMocks          map[string]interface{}
	generatedResources []*unstructured.Unstructured
	policyName         string
	policyNamespace    string
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
		resources:        map[string]map[string]map[string]*unstructured.Unstructured{},
		images:           map[string]map[string]any{},
		globalReferences: map[string]any{},
	}
}

func (cp *FakeContextProvider) AddImageData(image string, data map[string]any) {
	cp.images[image] = data
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

func (cp *FakeContextProvider) AddGlobalReference(name string, data any) {
	if cp.globalReferences == nil {
		cp.globalReferences = map[string]any{}
	}
	cp.globalReferences[name] = data
}

func (cp *FakeContextProvider) SetHTTPMocks(mocks map[string]interface{}) {
	cp.httpMocks = mocks
}

func (cp *FakeContextProvider) GetHTTPMocks() map[string]interface{} {
	return cp.httpMocks
}

func (cp *FakeContextProvider) GetGlobalReference(name, projection string) (any, error) {
	data, ok := cp.globalReferences[name]
	if !ok {
		return nil, nil
	}
	// When a projection is requested, look it up inside the stored data map.
	// This mirrors the real contextProvider.GetGlobalReference which calls
	// storeEntry.Get(projection) and returns only the projection value.
	if projection != "" {
		if m, ok := data.(map[string]interface{}); ok {
			if v, found := m[projection]; found {
				return v, nil
			}
			return nil, fmt.Errorf("projection %q not found in global context entry %q", projection, name)
		}
		return nil, fmt.Errorf("projection %q not found in global context entry %q: stored value is not an object", projection, name)
	}
	return data, nil
}

func (cp *FakeContextProvider) GetImageData(image string) (map[string]any, error) {
	if cp.images == nil {
		return nil, fmt.Errorf("image data not found in the context")
	}
	if _, found := cp.images[image]; !found {
		return nil, fmt.Errorf("image data for %s not found in the context", image)
	}
	return cp.images[image], nil
}

func (cp *FakeContextProvider) ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	panic("not implemented")
}

func (cp *FakeContextProvider) ListResources(apiVersion, resource, namespace string, labels map[string]string) (*unstructured.UnstructuredList, error) {
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

func (cp *FakeContextProvider) SetGenerateContext(polName, policyNamespace, triggerName, triggerNamespace, triggerAPIVersion, triggerGroup, triggerKind, triggerUID string, restoreCache bool) {
	cp.policyName = polName
	cp.policyNamespace = policyNamespace
	cp.triggerName = triggerName
	cp.triggerNamespace = triggerNamespace
	cp.triggerAPIVersion = triggerAPIVersion
	cp.triggerGroup = triggerGroup
	cp.triggerKind = triggerKind
	cp.triggerUID = triggerUID
	cp.restoreCache = restoreCache
}

func (f *FakeContextProvider) Clone() Context {
	// Returns a shallow copy. Maps, clients, and other referenced mutable state remain shared.
	// Only the copied top-level struct fields and the per-worker generatedResources list are isolated here.
	clone := *f

	// generatedResources is per-evaluation state. Ensure each worker starts with a clean slate.
	clone.generatedResources = make([]*unstructured.Unstructured, 0)

	return &clone
}
