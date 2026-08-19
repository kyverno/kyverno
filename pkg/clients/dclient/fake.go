package dclient

import (
	"errors"
	"fmt"
	"strings"

	openapiv2 "github.com/google/gnostic-models/openapiv2"
	"github.com/kyverno/kyverno/ext/wildcard"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// NewFakeClient ---testing utilities
func NewFakeClient(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) (Interface, error) {
	unstructuredScheme := runtime.NewScheme()
	for gvk := range scheme.AllKnownTypes() {
		if unstructuredScheme.Recognizes(gvk) {
			continue
		}
		if strings.HasSuffix(gvk.Kind, "List") {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
			continue
		}
		unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}
	objects, err := convertObjectsToUnstructured(objects)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !unstructuredScheme.Recognizes(gvk) {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		}
		gvk.Kind += "List"
		if !unstructuredScheme.Recognizes(gvk) {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
		}
	}
	c := fake.NewSimpleDynamicClientWithCustomListKinds(unstructuredScheme, gvrToListKind, objects...)
	// the typed and dynamic client are initialized with similar resources
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		dyn:  c,
		kube: kclient,
	}, nil
}

// NewFakeClientWithDisco creates a fake client with the given dynamic, kube, and discovery clients.
// Unlike NewClient, this does not start a background discovery cache polling goroutine.
func NewFakeClientWithDisco(dyn dynamic.Interface, kube kubernetes.Interface, disco IDiscovery) Interface {
	return &client{
		dyn:   dyn,
		disco: disco,
		kube:  kube,
	}
}

func NewEmptyFakeClient() Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{}
	objects := []runtime.Object{}
	scheme := runtime.NewScheme()
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		dyn:   fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...),
		disco: NewFakeDiscoveryClient(nil),
		kube:  kclient,
	}
}

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(registeredResources []schema.GroupVersionResource) *fakeDiscoveryClient {
	// Load some-preregistered resources
	res := []schema.GroupVersionResource{
		{Version: "v1", Resource: "configmaps"},
		{Version: "v1", Resource: "endpoints"},
		{Version: "v1", Resource: "namespaces"},
		{Version: "v1", Resource: "resourcequotas"},
		{Version: "v1", Resource: "secrets"},
		{Version: "v1", Resource: "serviceaccounts"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	registeredResources = append(registeredResources, res...)
	return &fakeDiscoveryClient{registeredResources: registeredResources}
}

type fakeDiscoveryClient struct {
	registeredResources []schema.GroupVersionResource
	gvrToGVK            map[schema.GroupVersionResource]schema.GroupVersionKind
}

func (c *fakeDiscoveryClient) AddGVRToGVKMapping(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) {
	if c.gvrToGVK == nil {
		c.gvrToGVK = make(map[schema.GroupVersionResource]schema.GroupVersionKind)
	}
	c.gvrToGVK[gvr] = gvk
}

func (c *fakeDiscoveryClient) getGVR(resource string) (schema.GroupVersionResource, error) {
	for _, gvr := range c.registeredResources {
		if gvr.Resource == resource {
			return gvr, nil
		}
	}
	return schema.GroupVersionResource{}, errors.New("not found")
}

func (c *fakeDiscoveryClient) GetGVKFromGVR(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	if c.gvrToGVK != nil {
		if gvk, exists := c.gvrToGVK[gvr]; exists {
			return gvk, nil
		}
	}

	for _, registered := range c.registeredResources {
		if registered.Group == gvr.Group && registered.Version == gvr.Version && registered.Resource == gvr.Resource {
			kind := inferKindFromResourceName(gvr.Resource)
			return schema.GroupVersionKind{
				Group:   gvr.Group,
				Version: gvr.Version,
				Kind:    kind,
			}, nil
		}
	}
	return schema.GroupVersionKind{}, fmt.Errorf("GVR not found: %s", gvr.String())
}

// inferKindFromResourceName converts a plural resource name to a singular kind
// e.g., "computeclasses" -> "ComputeClass", "pods" -> "Pod"
func inferKindFromResourceName(resource string) string {
	kind := resource
	if strings.HasSuffix(kind, "ies") {
		kind = strings.TrimSuffix(kind, "ies") + "y"
	} else if strings.HasSuffix(kind, "es") {
		kind = strings.TrimSuffix(kind, "es")
	} else if strings.HasSuffix(kind, "s") {
		kind = strings.TrimSuffix(kind, "s")
	}
	if len(kind) > 0 {
		kind = strings.ToUpper(kind[:1]) + kind[1:]
	}
	return kind
}

func (c *fakeDiscoveryClient) GetGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	// First try to find in reverse mapping (from gvrToGVK)
	if c.gvrToGVK != nil {
		for gvr, mappedGVK := range c.gvrToGVK {
			if mappedGVK.Group == gvk.Group && mappedGVK.Version == gvk.Version && mappedGVK.Kind == gvk.Kind {
				return gvr, nil
			}
		}
	}
	// Fallback: infer resource name from kind
	resource := strings.ToLower(gvk.Kind) + "s"
	return c.getGVR(resource)
}

var fakeConnectOnlySubresources = []string{"exec", "attach", "portforward"}

func (c *fakeDiscoveryClient) FindResources(group, version, kind, subresource string) (map[TopLevelApiDescription]metav1.APIResource, error) {
	subresources := []string{subresource}
	if subresource != "" && strings.Contains(subresource, "*") {
		subresources = fakeConnectOnlySubresources
	}

	results := map[TopLevelApiDescription]metav1.APIResource{}
	for _, resource := range c.registeredResources {
		if kind != "*" && resource.Resource != strings.ToLower(kind)+"s" {
			continue
		}
		desc := TopLevelApiDescription{
			GroupVersion: schema.GroupVersion{Group: resource.Group, Version: resource.Version},
			Kind:         inferKindFromResourceName(resource.Resource),
			Resource:     resource.Resource,
		}
		if subresource == "" {
			results[desc] = metav1.APIResource{Verbs: fakeVerbsFor("")}
			continue
		}
		if kind == "*" && subresource == "*" {
			// mirrors the real discovery client, which merges the plain
			// top-level resource in alongside its subresources here.
			results[desc] = metav1.APIResource{Verbs: fakeVerbsFor("")}
		}
		for _, sub := range subresources {
			if !wildcard.Match(subresource, sub) {
				continue
			}
			d := desc
			d.SubResource = sub
			results[d] = metav1.APIResource{Verbs: fakeVerbsFor(sub)}
		}
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return results, nil
}

// fakeVerbsFor approximates real API server verbs: connect-only
// subresources (exec/attach/portforward) get no get/list/watch, others
// behave like a status subresource.
func fakeVerbsFor(subresource string) []string {
	switch subresource {
	case "":
		return []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"}
	case "exec", "attach", "portforward":
		return []string{"create"}
	default:
		return []string{"get", "patch", "update"}
	}
}

func (c *fakeDiscoveryClient) OpenAPISchema() (*openapiv2.Document, error) {
	return nil, nil
}

func (c *fakeDiscoveryClient) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return nil
}

func (c *fakeDiscoveryClient) OnChanged(callback func()) {
	// No-op for fake client
}

func convertObjectsToUnstructured(objs []runtime.Object) ([]runtime.Object, error) {
	ul := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		u, err := kubeutils.ObjToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		ul = append(ul, u)
	}
	return ul, nil
}
