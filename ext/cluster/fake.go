package cluster

import (
	"context"
	"errors"
	"time"

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
						gvr[crdGVR] = crd.Spec.Names.Kind + "List"
						gvrToGVK[crdGVR] = schema.GroupVersionKind{
							Group:   crd.Spec.Group,
							Version: version.Name,
							Kind:    crd.Spec.Names.Kind,
						}
					}
				}
			}

			s.AddKnownTypeWithName(o.GetObjectKind().GroupVersionKind(), o)
			continue
		}

		plural, _ := meta.UnsafeGuessKindToResource(o.GetObjectKind().GroupVersionKind())
		if _, ok := gvr[plural]; ok {
			continue
		}

		s.AddKnownTypeWithName(o.GetObjectKind().GroupVersionKind(), o)

		gvr[plural] = o.GetObjectKind().GroupVersionKind().Kind + "List"
		gvrToGVK[plural] = o.GetObjectKind().GroupVersionKind()

		list = append(list, plural)
	}

	allFakeObjects := make([]runtime.Object, 0, len(objects))
	allFakeObjects = append(allFakeObjects, objects...)

	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(s, gvr, allFakeObjects...)

	// Filter out CRDs from objects before converting for kube client
	// CRDs are not regular Kubernetes resources and can't be converted
	kubeObjects := make([]runtime.Object, 0, len(objects))
	for _, o := range objects {
		if _, isCRD := o.(*apiextensionsv1.CustomResourceDefinition); !isCRD {
			kubeObjects = append(kubeObjects, o)
		}
	}
	kclient := kubefake.NewSimpleClientset(resource.ConvertResources(kubeObjects)...)

	dClient, _ := dclient.NewClient(context.Background(), dyn, kclient, time.Hour, false, nil)
	discoClient := dclient.NewFakeDiscoveryClient(list)
	for gvr, gvk := range gvrToGVK {
		discoClient.AddGVRToGVKMapping(gvr, gvk)
	}
	dClient.SetDiscovery(discoClient)

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
