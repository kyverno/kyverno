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
	"k8s.io/client-go/dynamic"
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

	// Collect CRDs in a first pass so we can build a REST mapper before the main
	// loop. The mapper lets us detect CRDs whose plural name does not match the
	// lowercase-kind + "s" heuristic (e.g. TeleportRoleV8: CRD plural
	// "teleportrolesv8", UnsafeGuessKindToResource gives "teleportrolev8s").
	// Without this, resource.List on the correct plural returns empty because
	// tracker.Add stores instances under the wrong key.
	var collectedCRDs []*apiextensionsv1.CustomResourceDefinition
	for _, o := range objects {
		if crd, ok := o.(*apiextensionsv1.CustomResourceDefinition); ok {
			collectedCRDs = append(collectedCRDs, crd)
		}
	}
	crdAPIGroups := make([]*restmapper.APIGroupResources, 0, len(collectedCRDs))
	for _, crd := range collectedCRDs {
		crdAPIGroups = append(crdAPIGroups, convertCRDToAPIGroupResources(crd))
	}
	crdMapper := restmapper.NewDiscoveryRESTMapper(crdAPIGroups)

	for _, o := range objects {
		if crd, ok := o.(*apiextensionsv1.CustomResourceDefinition); ok {
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
			continue
		}

		gvk := o.GetObjectKind().GroupVersionKind()
		plural, _ := meta.UnsafeGuessKindToResource(gvk)
		if _, ok := gvr[plural]; ok {
			continue
		}

		s.AddKnownTypeWithName(gvk, o)
		gvkList := gvk
		gvkList.Kind += "List"
		if !s.Recognizes(gvkList) {
			s.AddKnownTypeWithName(gvkList, &unstructured.UnstructuredList{})
		}

		gvr[plural] = gvkList.Kind
		gvrToGVK[plural] = gvk

		list = append(list, plural)
	}

	// Build a remap table: CRD-declared GVR → guessed GVR (where tracker.Add
	// actually stores objects). tracker.Add uses UnsafeGuessKindToResource, so
	// for CRDs with non-standard plural names the two diverge. We wrap the
	// dynamic interface below so that callers using the correct CRD plural are
	// transparently redirected to the GVR the tracker knows about.
	gvrRemap := make(map[schema.GroupVersionResource]schema.GroupVersionResource)
	for _, o := range objects {
		obj, ok := o.(*unstructured.Unstructured)
		if !ok {
			continue
		}
		gvk := obj.GroupVersionKind()
		guessed, _ := meta.UnsafeGuessKindToResource(gvk)
		mapping, err := crdMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil || mapping.Resource == guessed {
			continue
		}
		gvrRemap[mapping.Resource] = guessed // teleportrolesv8 → teleportrolev8s
	}

	allFakeObjects := make([]runtime.Object, 0, len(objects))
	allFakeObjects = append(allFakeObjects, objects...)

	dyn := newRemappingDynamic(
		fake.NewSimpleDynamicClientWithCustomListKinds(s, gvr, allFakeObjects...),
		gvrRemap,
	)

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

// remappingDynamic wraps a dynamic.Interface and redirects Resource() calls for
// CRD-declared GVRs to the guessed GVR that tracker.Add actually used as the
// storage key. This makes retrieval consistent with storage without requiring a
// separate Create pass or any changes to how objects are loaded.
type remappingDynamic struct {
	dynamic.Interface
	remap map[schema.GroupVersionResource]schema.GroupVersionResource
}

func newRemappingDynamic(dyn dynamic.Interface, remap map[schema.GroupVersionResource]schema.GroupVersionResource) dynamic.Interface {
	if len(remap) == 0 {
		return dyn
	}
	return &remappingDynamic{Interface: dyn, remap: remap}
}

func (d *remappingDynamic) Resource(gvr schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	if mapped, ok := d.remap[gvr]; ok {
		gvr = mapped
	}
	return d.Interface.Resource(gvr)
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
