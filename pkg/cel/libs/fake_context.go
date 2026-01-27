package libs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	celhttp "github.com/kyverno/kyverno/pkg/cel/libs/http"
	"k8s.io/apimachinery/pkg/api/meta"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FakeContextProvider struct {
	resources          map[string]map[string]map[string]*unstructured.Unstructured
	images             map[string]map[string]any
	globalContext      map[string]map[string]any
	restMapper         meta.RESTMapper
	httpClient         *fakeHTTPClient
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
		resources:     map[string]map[string]map[string]*unstructured.Unstructured{},
		images:        map[string]map[string]any{},
		globalContext: map[string]map[string]any{},
		httpClient:    &fakeHTTPClient{stubs: map[string]httpStub{}},
	}
}

func (cp *FakeContextProvider) AddImageData(image string, data map[string]any) {
	cp.images[image] = data
}

func (cp *FakeContextProvider) SetRESTMapper(mapper meta.RESTMapper) {
	cp.restMapper = mapper
}

func (cp *FakeContextProvider) AddGlobalReference(name, projection string, value any) {
	if cp.globalContext == nil {
		cp.globalContext = map[string]map[string]any{}
	}
	entry := cp.globalContext[name]
	if entry == nil {
		entry = map[string]any{}
		cp.globalContext[name] = entry
	}
	entry[projection] = value
}

type httpStub struct {
	status  int
	headers map[string]string
	body    []byte
}

type fakeHTTPClient struct {
	stubs map[string]httpStub
}

func (c *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c == nil || c.stubs == nil {
		return nil, fmt.Errorf("no HTTP stubs configured - add HTTP stubs to your context file to test policies that use kyverno.http CEL library")
	}
	key := strings.ToUpper(req.Method) + " " + req.URL.String()
	stub, ok := c.stubs[key]
	if !ok {
		availableStubs := make([]string, 0, len(c.stubs))
		for k := range c.stubs {
			availableStubs = append(availableStubs, k)
		}
		return nil, fmt.Errorf("no HTTP stub found for %s - available stubs: %v. Add a matching stub to your context file", key, availableStubs)
	}
	status := stub.status
	if status == 0 {
		status = http.StatusOK
	}
	body := stub.body
	if body == nil {
		body = []byte("null")
	}
	resp := &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")
	for k, v := range stub.headers {
		resp.Header.Set(k, v)
	}
	return resp, nil
}

func (cp *FakeContextProvider) AddHTTPStub(method, url string, status int, headers map[string]string, body []byte) {
	if cp.httpClient == nil {
		cp.httpClient = &fakeHTTPClient{stubs: map[string]httpStub{}}
	}
	if cp.httpClient.stubs == nil {
		cp.httpClient.stubs = map[string]httpStub{}
	}
	key := strings.ToUpper(method) + " " + url
	cp.httpClient.stubs[key] = httpStub{
		status:  status,
		headers: headers,
		body:    body,
	}
}

// HTTPClient returns a client suitable for the kyverno.http CEL library.
func (cp *FakeContextProvider) HTTPClient() celhttp.ClientInterface {
	if cp.httpClient == nil {
		cp.httpClient = &fakeHTTPClient{stubs: map[string]httpStub{}}
	}
	return cp.httpClient
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

func (cp *FakeContextProvider) GetGlobalReference(name, projection string) (any, error) {
	if cp.globalContext == nil {
		return nil, nil
	}
	entry, ok := cp.globalContext[name]
	if !ok {
		// match real provider behavior: missing entry -> nil, nil
		return nil, nil
	}
	if value, ok := entry[projection]; ok {
		return value, nil
	}
	// fall back to the default projection when available
	if projection != "" {
		if value, ok := entry[""]; ok {
			return value, nil
		}
	}
	availableProjections := make([]string, 0, len(entry))
	for p := range entry {
		if p == "" {
			availableProjections = append(availableProjections, "(default)")
		} else {
			availableProjections = append(availableProjections, p)
		}
	}
	return nil, fmt.Errorf("global context entry %q projection %q not found - available projections: %v. Add the missing projection to your context file", name, projection, availableProjections)
}

func (cp *FakeContextProvider) GetImageData(image string) (map[string]any, error) {
	if cp.images == nil {
		return nil, fmt.Errorf("image data not found in the context - add image data to your context file to test policies that use image verification")
	}
	if _, found := cp.images[image]; !found {
		availableImages := make([]string, 0, len(cp.images))
		for img := range cp.images {
			availableImages = append(availableImages, img)
		}
		return nil, fmt.Errorf("image data for %s not found in the context - available images: %v. Add the missing image data to your context file", image, availableImages)
	}
	return cp.images[image], nil
}

func (cp *FakeContextProvider) ToGVR(apiVersion, kind string) (*schema.GroupVersionResource, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	if cp.restMapper != nil {
		mapping, err := cp.restMapper.RESTMapping(schema.GroupKind{Group: gv.Group, Kind: kind}, gv.Version)
		if err == nil {
			out := mapping.Resource
			return &out, nil
		}
	}
	// best-effort fallback when no rest mapper is available
	resource := strings.ToLower(kind) + "s"
	out := schema.GroupVersionResource{
		Group:    gv.Group,
		Version:  gv.Version,
		Resource: resource,
	}
	return &out, nil
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
		if len(labels) != 0 {
			objLabels := obj.GetLabels()
			matches := true
			for k, v := range labels {
				if objLabels == nil || objLabels[k] != v {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
		}
		out.Items = append(out.Items, *obj.DeepCopy())
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

func (cp *FakeContextProvider) PostResource(apiVersion, resource, namespace string, data map[string]any) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	gvr := gv.WithResource(resource)
	obj := &unstructured.Unstructured{Object: data}
	if namespace != "" {
		obj.SetNamespace(namespace)
	}
	name := obj.GetName()
	if name == "" {
		return nil, fmt.Errorf("failed to create %s: missing metadata.name", gvr.String())
	}
	// normalize by encoding/decoding through JSON (ensures map[string]any types are consistent)
	raw, err := json.Marshal(obj.Object)
	if err != nil {
		return nil, err
	}
	var normalized map[string]any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return nil, err
	}
	obj.Object = normalized

	resources := cp.resources[gvr.String()]
	if resources == nil {
		resources = map[string]map[string]*unstructured.Unstructured{}
		cp.resources[gvr.String()] = resources
	}
	nsMap := resources[obj.GetNamespace()]
	if nsMap == nil {
		nsMap = map[string]*unstructured.Unstructured{}
		resources[obj.GetNamespace()] = nsMap
	}
	nsMap[name] = obj
	return obj.DeepCopy(), nil
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
