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
	resourceScopes      map[schema.GroupVersionResource]bool
	preferredVersions   map[string]string
}

// SetResourceScope supplies discovery scope for an offline resource fixture.
func (c *fakeDiscoveryClient) SetResourceScope(gvr schema.GroupVersionResource, namespaced bool) {
	if c.resourceScopes == nil {
		c.resourceScopes = make(map[schema.GroupVersionResource]bool)
	}
	c.resourceScopes[gvr] = namespaced
}

func (c *fakeDiscoveryClient) AddGVRToGVKMapping(gvr schema.GroupVersionResource, gvk schema.GroupVersionKind) {
	if c.gvrToGVK == nil {
		c.gvrToGVK = make(map[schema.GroupVersionResource]schema.GroupVersionKind)
	}
	c.gvrToGVK[gvr] = gvk
}

// SetPreferredVersion supplies the discovery preference used when a version is omitted.
func (c *fakeDiscoveryClient) SetPreferredVersion(group, version string) {
	if c.preferredVersions == nil {
		c.preferredVersions = make(map[string]string)
	}
	c.preferredVersions[group] = version
}

func (c *fakeDiscoveryClient) getGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	resource := strings.ToLower(gvk.Kind) + "s"
	for _, gvr := range c.registeredResources {
		// Explicit mappings are authoritative, including irregular resource names.
		if _, mapped := c.gvrToGVK[gvr]; mapped {
			continue
		}
		if gvr.Resource == resource &&
			(gvk.Group == "" && gvk.Version == "" || gvr.Group == gvk.Group) &&
			(gvk.Version == "" || gvr.Version == gvk.Version) {
			// Unmapped test fixtures retain their explicit registration-order preference.
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
	var matches []schema.GroupVersionResource
	for gvr, mappedGVK := range c.gvrToGVK {
		if mappedGVK.Kind == gvk.Kind &&
			(gvk.Group == "" && gvk.Version == "" || mappedGVK.Group == gvk.Group) &&
			(gvk.Version == "" || mappedGVK.Version == gvk.Version) {
			matches = append(matches, gvr)
		}
	}
	if len(matches) > 1 && gvk.Group == "" && gvk.Version == "" {
		// The runtime discovery RESTMapper gives core/v1 priority for kind-only lookups.
		var core []schema.GroupVersionResource
		for _, gvr := range matches {
			if gvr.Group == "" && gvr.Version == "v1" {
				core = append(core, gvr)
			}
		}
		if len(core) != 0 {
			matches = core
		}
	}
	if len(matches) > 1 && gvk.Version == "" {
		var preferred []schema.GroupVersionResource
		for _, gvr := range matches {
			if version := c.preferredVersions[gvr.Group]; version == "" || version == gvr.Version {
				preferred = append(preferred, gvr)
			}
		}
		matches = preferred
	}
	switch len(matches) {
	case 0:
		return c.getGVR(gvk)
	case 1:
		return matches[0], nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("ambiguous resource for %s: specify a group and version", gvk)
	}
}

func (c *fakeDiscoveryClient) FindResources(group, version, kind, subresource string) (map[TopLevelApiDescription]metav1.APIResource, error) {
	r := strings.ToLower(kind) + "s"
	resources := make(map[TopLevelApiDescription]metav1.APIResource)
	for _, resource := range c.registeredResources {
		if !wildcard.Match(group, resource.Group) || !wildcard.Match(version, resource.Version) {
			continue
		}
		resourceKind := kind
		matchesKind := wildcard.Match(r, resource.Resource)
		if gvk, mapped := c.gvrToGVK[resource]; mapped {
			resourceKind = gvk.Kind
			matchesKind = wildcard.Match(kind, resourceKind)
		} else if wildcard.ContainsWildcard(kind) {
			resourceKind = inferKindFromResourceName(resource.Resource)
		}
		if matchesKind {
			resources[TopLevelApiDescription{
				GroupVersion: resource.GroupVersion(),
				Kind:         resourceKind,
				Resource:     resource.Resource,
				SubResource:  subresource,
			}] = metav1.APIResource{Namespaced: c.resourceScopes[resource]}
		}
	}
	if len(resources) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return resources, nil
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
