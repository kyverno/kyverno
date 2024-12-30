package fuzz

import (
	"context"
	"fmt"
	"io"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	openapiv2 "github.com/google/gnostic-models/openapiv2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	eventsv1 "k8s.io/client-go/kubernetes/typed/events/v1"
)

type FuzzInterface struct {
	FF *fuzz.ConsumeFuzzer
}

func (fi FuzzInterface) GetKubeClient() kubernetes.Interface {
	return nil
}

func (fi FuzzInterface) GetEventsInterface() eventsv1.EventsV1Interface {
	return nil
}

func (fi FuzzInterface) GetDynamicInterface() dynamic.Interface {
	return DynamicFuzz{
		ff: fi.FF,
	}
}

func (fi FuzzInterface) Discovery() dclient.IDiscovery {
	return FuzzIDiscovery{ff: fi.FF}
}

func (fi FuzzInterface) SetDiscovery(discoveryClient dclient.IDiscovery) {
}

func (fi FuzzInterface) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return []byte("fuzz"), fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) GetResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) PatchResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, patch []byte) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ListResource(ctx context.Context, apiVersion string, kind string, namespace string, lselector *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) DeleteResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, dryRun bool) error {
	return fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) CreateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) UpdateResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) UpdateStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, obj interface{}, dryRun bool) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ApplyResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fi FuzzInterface) ApplyStatusResource(ctx context.Context, apiVersion string, kind string, namespace string, name string, obj interface{}, dryRun bool, fieldManager string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

type FuzzIDiscovery struct {
	ff *fuzz.ConsumeFuzzer
}

func (fid FuzzIDiscovery) FindResources(group, version, kind, subresource string) (map[dclient.TopLevelApiDescription]metav1.APIResource, error) {
	noOfRes, err := fid.ff.GetInt()
	if err != nil {
		return nil, err
	}
	m := make(map[dclient.TopLevelApiDescription]metav1.APIResource)
	for i := 0; i < noOfRes%10; i++ {
		gvGroup, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvVersion, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelKind, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		topLevelSubResource, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		gvr := schema.GroupVersionResource{
			Group:    gvGroup,
			Version:  gvVersion,
			Resource: gvResource,
		}
		topLevel := dclient.TopLevelApiDescription{
			GroupVersion: gvr.GroupVersion(),
			Kind:         topLevelKind,
			Resource:     topLevelResource,
			SubResource:  topLevelSubResource,
		}
		apiResource := metav1.APIResource{}
		apiName, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		apiResource.Name = apiName

		apiSingularName, err := GetK8sString(fid.ff)
		if err != nil {
			return nil, err
		}
		apiResource.SingularName = apiSingularName

		namespaced, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		apiResource.Namespaced = namespaced

		setGroup, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		if setGroup {
			apiGroup, err := GetK8sString(fid.ff)
			if err != nil {
				return nil, err
			}
			apiResource.Group = apiGroup
		}

		verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection", "proxy"}
		apiResource.Verbs = verbs

		setShortNames, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		shortNames := make([]string, 0)
		if setShortNames {
			err = fid.ff.CreateSlice(&shortNames)
			if err != nil {
				return nil, err
			}
			apiResource.ShortNames = shortNames
		}
		setCategories, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		categories := make([]string, 0)
		if setCategories {
			err = fid.ff.CreateSlice(&categories)
			if err != nil {
				return nil, err
			}
			apiResource.Categories = categories
		}

		setStorageHash, err := fid.ff.GetBool()
		if err != nil {
			return nil, err
		}
		if setStorageHash {
			storageHash, err := fid.ff.GetString()
			if err != nil {
				return nil, err
			}
			apiResource.StorageVersionHash = storageHash
		}
		m[topLevel] = apiResource
	}
	return m, nil
}

func (fid FuzzIDiscovery) GetGVRFromGVK(schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{}, fmt.Errorf("Not implemented")
}

func (fid FuzzIDiscovery) GetGVKFromGVR(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, fmt.Errorf("Not implemented")
}

func (fid FuzzIDiscovery) OpenAPISchema() (*openapiv2.Document, error) {
	b, err := fid.ff.GetBytes()
	if err != nil {
		return nil, err
	}
	return openapiv2.ParseDocument(b)
}

func (fid FuzzIDiscovery) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return nil
}

type DynamicFuzz struct {
	ff *fuzz.ConsumeFuzzer
}

func (df DynamicFuzz) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return FuzzNamespaceableResource(df)
}

type FuzzResource struct {
	ff *fuzz.ConsumeFuzzer
}

func (fr FuzzResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("Not implemented")
}

func (fr FuzzResource) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	resource, err := CreateUnstructuredObject(fr.ff, "")
	if err != nil {
		return nil, err
	}

	return resource, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

type FuzzNamespaceableResource struct {
	ff *fuzz.ConsumeFuzzer
}

func (fnr FuzzNamespaceableResource) Namespace(string) dynamic.ResourceInterface {
	return FuzzResource(fnr)
}

func (fr FuzzNamespaceableResource) Create(ctx context.Context, obj *unstructured.Unstructured, options metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) Update(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) UpdateStatus(ctx context.Context, obj *unstructured.Unstructured, options metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) Delete(ctx context.Context, name string, options metav1.DeleteOptions, subresources ...string) error {
	return fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) DeleteCollection(ctx context.Context, options metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	return fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) Get(ctx context.Context, name string, options metav1.GetOptions, subresources ...string) (*unstructured.Unstructured, error) {
	resource, err := CreateUnstructuredObject(fr.ff, "")
	if err != nil {
		return nil, err
	}

	return resource, nil
}

func (fr FuzzNamespaceableResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	var objs []unstructured.Unstructured
	objs = make([]unstructured.Unstructured, 0)
	noOfObjs, err := fr.ff.GetInt()
	if err != nil {
		return nil, err
	}
	for i := 0; i < noOfObjs%10; i++ {
		obj, err := CreateUnstructuredObject(fr.ff, "")
		if err != nil {
			return nil, err
		}
		objs = append(objs, *obj)
	}
	return &unstructured.UnstructuredList{
		Object: map[string]interface{}{"kind": "List", "apiVersion": "v1"},
		Items:  objs,
	}, nil
}

func (fr FuzzNamespaceableResource) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, options metav1.PatchOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) Apply(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions, subresources ...string) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}

func (fr FuzzNamespaceableResource) ApplyStatus(ctx context.Context, name string, obj *unstructured.Unstructured, options metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	return nil, fmt.Errorf("Not implemented")
}
