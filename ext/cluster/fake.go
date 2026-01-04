package cluster

import (
	"context"
	"errors"
	"time"

	v2 "github.com/kyverno/kyverno/api/kyverno/v2"
	kdata "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/data"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
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

	"github.com/kyverno/playground/backend/data"
	"github.com/kyverno/playground/backend/pkg/resource"
	"github.com/kyverno/playground/backend/pkg/utils"
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

func (c fakeCluster) PolicyExceptionSelector(namespace string, exceptions ...*v2.PolicyException) engineapi.PolicyExceptionSelector {
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

func (c fakeCluster) DClient(resources []runtime.Object, objects ...runtime.Object) (dclient.Interface, error) {
	s := runtime.NewScheme()
	gvr := make(map[schema.GroupVersionResource]string)
	list := []schema.GroupVersionResource{}

	for _, o := range resources {
		plural, _ := meta.UnsafeGuessKindToResource(o.GetObjectKind().GroupVersionKind())
		if _, ok := gvr[plural]; ok {
			continue
		}

		s.AddKnownTypeWithName(o.GetObjectKind().GroupVersionKind(), o)

		gvr[plural] = o.GetObjectKind().GroupVersionKind().Kind + "List"

		list = append(list, plural)
	}

	for _, o := range objects {
		plural, _ := meta.UnsafeGuessKindToResource(o.GetObjectKind().GroupVersionKind())
		if _, ok := gvr[plural]; ok {
			continue
		}

		s.AddKnownTypeWithName(o.GetObjectKind().GroupVersionKind(), o)

		gvr[plural] = o.GetObjectKind().GroupVersionKind().Kind + "List"

		list = append(list, plural)
	}

	dyn := fake.NewSimpleDynamicClientWithCustomListKinds(s, gvr, objects...)
	kclient := kubefake.NewSimpleClientset(resource.ConvertResources(objects)...)

	dClient, _ := dclient.NewClient(context.Background(), dyn, kclient, time.Hour, false, nil)
	dClient.SetDiscovery(dclient.NewFakeDiscoveryClient(list))

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
