package policy

import (
	"context"
	"encoding/json"
	"errors"

	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/config"
	gctxstore "github.com/kyverno/kyverno/pkg/globalcontext/store"
	"github.com/kyverno/kyverno/pkg/imagedataloader"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

type Context = contextlib.ContextInterface

type contextProvider struct {
	client    kubernetes.Interface
	imagedata imagedataloader.Fetcher
	gctxStore gctxstore.Store
}

func NewContextProvider(
	client kubernetes.Interface,
	imageOpts []imagedataloader.Option,
	gctxStore gctxstore.Store,
) (Context, error) {
	idl, err := imagedataloader.New(client.CoreV1().Secrets(config.KyvernoNamespace()), imageOpts...)
	if err != nil {
		return nil, err
	}
	return &contextProvider{
		client:    client,
		imagedata: idl,
		gctxStore: gctxStore,
	}, nil
}

func (cp *contextProvider) GetConfigMap(namespace string, name string) (unstructured.Unstructured, error) {
	cm, err := cp.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	out, err := kubeutils.ObjToUnstructured(cm)
	if err != nil {
		return unstructured.Unstructured{}, err
	}
	return *out, nil
}

func (cp *contextProvider) GetGlobalReference(name, _ string) (any, error) {
	ent, ok := cp.gctxStore.Get(name)
	if !ok {
		return nil, errors.New("global context entry not found")
	}
	data, err := ent.Get()
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

func (cp *contextProvider) GetImageData(image string) (*imagedataloader.ImageData, error) {
	// TODO: get image credentials from image verification policies?
	return cp.imagedata.FetchImageData(context.TODO(), image)
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
