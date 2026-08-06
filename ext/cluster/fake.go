package cluster

import (
	"context"
	"errors"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kdata "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/playground/backend/data"
	"github.com/kyverno/playground/backend/pkg/resource"
	"github.com/kyverno/playground/backend/pkg/utils"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/openapi"
	"k8s.io/client-go/restmapper"
	"sigs.k8s.io/kubectl-validate/pkg/openapiclient"
)

type fakeCluster struct{}

func NewFake() Cluster {
	return fakeCluster{}
}

func (c fakeCluster) Kinds(_ context.Context, excludeGroups ...string) ([]Resource, error) {
	return nil, errors.New("listing kinds not supported in fake cluster")
}

func (c fakeCluster) Namespaces(ctx context.Context) ([]string, error) {
	return nil, errors.New("listing namespaces not supported in fake cluster")
}

func (c fakeCluster) Search(ctx context.Context, apiVersion string, kind string, namespace string, labels map[string]string) ([]SearchResult, error) {
	return nil, errors.New("searching resources not supported in fake cluster")
}

func (c fakeCluster) Get(ctx context.Context, apiVersion string, kind string, namespace string, name string) (*unstructured.Unstructured, error) {
	return nil, errors.New("getting resource not supported in fake cluster")
}

func (c fakeCluster) PolicyExceptionSelector(namespace string, exceptions ...*kyvernov2.PolicyException) engineapi.PolicyExceptionSelector {
	return NewPolicyExceptionSelector(namespace, nil, exceptions...)
}

func (c fakeCluster) OpenAPIClient(version string) (openapi.Client, error) {
	kubeVersion, err := utils.ParseKubeVersion(version)
	if err != nil {
		return nil, err
	}
	schemas, err := data.Schemas()
	if err != nil {
		return nil, err
	}

	return openapiclient.NewComposite(
		openapiclient.NewHardcodedBuiltins(kubeVersion),
		openapiclient.NewLocalSchemaFiles(schemas),
	), nil
}

func (c fakeCluster) DClient(objects []runtime.Object) (dclient.Interface, error) {
	s := runtime.NewScheme()
	gvr := make(map[schema.GroupVersionResource]string)
	list := []schema.GroupVersionResource{}
	gvrToGVK := make(map[schema.GroupVersionResource]schema.GroupVersionKind)

	// Pass 1: register every CRD and collect them so we can build a REST mapper
	// before processing CR instances. This is needed because the object list may
	// contain instances before their CRD, and the order must not matter.
	var collectedCRDs []*apiextensionsv1.CustomResourceDefinition
	for _, o := range objects {
		crd, ok := o.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			continue
		}
		collectedCRDs = append(collectedCRDs, crd)
		for _, version := range crd.Spec.Versions {
			if version.Storage {
				crdGVR := schema.GroupVersionResource{
					Group:    crd.Spec.Group,
					Version:  version.Name,
					Resource: crd.Spec.Names.Plural,
				}
				if _, exists := gvr[crdGVR]; !exists {
					list = append(list, crdGVR)
					crdGVK := schema.GroupVersionKind{
						Group:   crd.Spec.Group,
						Version: version.Name,
						Kind:    crd.Spec.Names.Kind,
					}
					gvr[crdGVR] = crdGVK.Kind + "List"
					gvrToGVK[crdGVR] = crdGVK

					crdGVKList := crdGVK
					crdGVKList.Kind += "List"
					if !s.Recognizes(crdGVKList) {
						s.AddKnownTypeWithName(crdGVKList, &unstructured.UnstructuredList{})
					}
				}
			}
		}

		s.AddKnownTypeWithName(o.GetObjectKind().GroupVersionKind(), o)
	}

	// Build a REST mapper from the collected CRDs. This lets pass 2 resolve the
	// GVR for CR instances using the CRD-declared plural rather than the heuristic
	// meta.UnsafeGuessKindToResource, which produces the wrong plural for resources
	// like TeleportRoleV8 (CRD plural "teleportrolesv8", guessed "teleportrolev8s").
	var crdAPIGroups []*restmapper.APIGroupResources
	for _, crd := range collectedCRDs {
		crdAPIGroups = append(crdAPIGroups, convertCRDToAPIGroupResources(crd))
	}
	crdMapper := restmapper.NewDiscoveryRESTMapper(crdAPIGroups)

	// Pass 2: register non-CRD object types in the scheme and GVR maps.
	// Objects are NOT added to allFakeObjects here; they are inserted into the
	// tracker via Create below so that the explicit CRD-declared GVR is used as
	// the storage key rather than the UnsafeGuessKindToResource result.
	for _, o := range objects {
		if _, ok := o.(*apiextensionsv1.CustomResourceDefinition); ok {
			continue
		}
		gvk := o.GetObjectKind().GroupVersionKind()

		// Determine the correct GVR: prefer the CRD-declared plural over the guess.
		var gvrKey schema.GroupVersionResource
		if mapping, err := crdMapper.RESTMapping(gvk.GroupKind(), gvk.Version); err == nil {
			gvrKey = mapping.Resource
		} else {
			gvrKey, _ = meta.UnsafeGuessKindToResource(gvk)
		}

		// Always register the GVK in the scheme; the tracker needs it for List.
		if !s.Recognizes(gvk) {
			s.AddKnownTypeWithName(gvk, o)
		}
		gvkList := gvk
		gvkList.Kind += "List"
		if !s.Recognizes(gvkList) {
			s.AddKnownTypeWithName(gvkList, &unstructured.UnstructuredList{})
		}

		// Only add a new GVR entry if the CRD pass hasn't registered it already.
		if _, ok := gvr[gvrKey]; !ok {
			gvr[gvrKey] = gvkList.Kind
			gvrToGVK[gvrKey] = gvk
			list = append(list, gvrKey)
		}
	}

	// Create the fake dynamic client with the scheme and GVR map but without
	// pre-loading objects. Objects are added via Create below so they are stored
	// in the tracker under the correct GVR (the one from the CRD or the fallback
	// guess) rather than the UnsafeGuessKindToResource result that tracker.Add
	// would use internally.
	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(s, gvr)

	// Insert each non-CRD object into the tracker under the correct GVR using
	// Create, which routes through ObjectReaction → tracker.Create(gvr, obj, ns)
	// and stores the object under the explicit GVR from dyn.Resource(gvrKey).
	for _, o := range objects {
		if _, ok := o.(*apiextensionsv1.CustomResourceDefinition); ok {
			continue
		}
		obj, ok := o.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		gvk := obj.GroupVersionKind()
		var objGVR schema.GroupVersionResource
		if mapping, err := crdMapper.RESTMapping(gvk.GroupKind(), gvk.Version); err == nil {
			objGVR = mapping.Resource
		} else {
			objGVR, _ = meta.UnsafeGuessKindToResource(gvk)
		}
		ns := obj.GetNamespace()
		ri := dyn.Resource(objGVR)
		if ns != "" {
			if _, err := ri.Namespace(ns).Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
				return nil, err
			}
		} else {
			if _, err := ri.Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
				return nil, err
			}
		}
	}

	// Filter out CRDs from objects before converting for kube client
	// CRDs are not regular Kubernetes resources and can't be converted
	kubeObjects := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		if _, isCRD := o.(*apiextensionsv1.CustomResourceDefinition); !isCRD {
			kubeObjects = append(kubeObjects, o)
		}
	}
	kclient := kubefake.NewSimpleClientset(resource.ConvertResources(kubeObjects)...)

	discoClient := dclient.NewFakeDiscoveryClient(list)
	for gvr, gvk := range gvrToGVK {
		discoClient.AddGVRToGVKMapping(gvr, gvk)
	}
	dClient := dclient.NewFakeClientWithDisco(dyn, kclient, discoClient)

	return dClient, nil
}

func (c fakeCluster) RESTMapper(crds []*apiextensionsv1.CustomResourceDefinition) meta.RESTMapper {
	apiGroupResources, _ := kdata.APIGroupResources()
	for _, crd := range crds {
		apiGroupResources = append(apiGroupResources, convertCRDToAPIGroupResources(crd))
	}

	return restmapper.NewDiscoveryRESTMapper(apiGroupResources)
}

func (c fakeCluster) IsFake() bool {
	return true
}

func convertCRDToAPIGroupResources(crd *apiextensionsv1.CustomResourceDefinition) *restmapper.APIGroupResources {
	groupResources := &restmapper.APIGroupResources{
		Group: metav1.APIGroup{
			Name:             crd.Spec.Group,
			Versions:         []metav1.GroupVersionForDiscovery{},
			PreferredVersion: metav1.GroupVersionForDiscovery{},
		},
		VersionedResources: make(map[string][]metav1.APIResource),
	}

	for _, v := range crd.Spec.Versions {
		groupResources.Group.Versions = append(groupResources.Group.Versions, metav1.GroupVersionForDiscovery{
			GroupVersion: crd.Spec.Group + "/" + v.Name,
			Version:      v.Name,
		})
		if v.Storage {
			groupResources.Group.PreferredVersion.GroupVersion = crd.Spec.Group + "/" + v.Name
			groupResources.Group.PreferredVersion.Version = v.Name
		}

		groupResources.VersionedResources[v.Name] = []metav1.APIResource{
			{
				Name:         crd.Spec.Names.Plural,
				SingularName: crd.Spec.Names.Singular,
				Namespaced:   crd.Spec.Scope == apiextensionsv1.NamespaceScoped,
				Kind:         crd.Spec.Names.Kind,
				Verbs:        metav1.Verbs{"get", "list", "watch", "create", "update", "patch", "delete"},
				ShortNames:   crd.Spec.Names.ShortNames,
			},
		}
	}
	return groupResources
}
