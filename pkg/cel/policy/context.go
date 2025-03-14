package policy

import (
	"context"
	"encoding/json"
	"errors"

	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imageverification/imagedataloader"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Context = contextlib.ContextInterface

type contextProvider struct {
	client    kubernetes.Interface
	dclient   dynamic.Interface
	imagedata imagedataloader.Fetcher
	gctxStore gctxstore.Store
}

func NewContextProvider(
	client dclient.Interface,
	imageOpts []imagedataloader.Option,
	gctxStore gctxstore.Store,
) (Context, error) {
	idl, err := imagedataloader.New(client.GetKubeClient().CoreV1().Secrets(config.KyvernoNamespace()), imageOpts...)
	if err != nil {
		return nil, err
	}
	return &contextProvider{
		client:    client.GetKubeClient(),
		dclient:   client.GetDynamicInterface(),
		imagedata: idl,
		gctxStore: gctxStore,
	}, nil
}

func (cp *contextProvider) GetConfigMap(namespace string, name string) (*unstructured.Unstructured, error) {
	cm, err := cp.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	out, err := kubeutils.ObjToUnstructured(cm)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (cp *contextProvider) GetGlobalReference(name, projection string) (any, error) {
	ent, ok := cp.gctxStore.Get(name)
	if !ok {
		return nil, errors.New("global context entry not found")
	}
	data, err := ent.Get(projection)
	if err != nil {
		return nil, err
	}

	if isLikelyKubernetesObject(data) {
		out, err := kubeutils.ObjToUnstructured(data)
		if err != nil {
			return nil, err
		}
		if out != nil {
			return *out, nil
		} else {
			return nil, errors.New("failed to convert to Unstructured")
		}
	} else {
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		apiData := map[string]interface{}{}
		err = json.Unmarshal(raw, &apiData)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
}

func (cp *contextProvider) GetImageData(image string) (map[string]interface{}, error) {
	// TODO: get image credentials from image verification policies?
	data, err := cp.imagedata.FetchImageData(context.TODO(), image)
	if err != nil {
		return nil, err
	}
	return getValue(data.Data())
}

func isLikelyKubernetesObject(data any) bool {
	if data == nil {
		return false
	}

	if m, ok := data.(map[string]interface{}); ok {
		_, hasAPIVersion := m["apiVersion"]
		_, hasKind := m["kind"]
		return hasAPIVersion && hasKind
	}

	if _, ok := data.(runtime.Object); ok {
		return true
	}

	return false
}

func (cp *contextProvider) ListResources(apiVersion, resource, namespace string) (*unstructured.UnstructuredList, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.List(context.TODO(), metav1.ListOptions{})
}

func (cp *contextProvider) GetResource(apiVersion, resource, namespace, name string) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, err
	}
	resourceInteface := cp.getResourceClient(groupVersion, resource, namespace)
	return resourceInteface.Get(context.TODO(), name, metav1.GetOptions{})
}

func (cp *contextProvider) getResourceClient(groupVersion schema.GroupVersion, resource string, namespace string) dynamic.ResourceInterface {
	client := cp.dclient.Resource(groupVersion.WithResource(resource))
	if namespace != "" {
		return client.Namespace(namespace)
	} else {
		return client
	}
}
