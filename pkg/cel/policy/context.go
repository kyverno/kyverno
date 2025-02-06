package policy

import (
	"context"

	contextlib "github.com/kyverno/kyverno/pkg/cel/libs/context"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/imagedataloader"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

type Context = contextlib.ContextInterface

type contextProvider struct {
	client    kubernetes.Interface
	imagedata imagedataloader.Fetcher
}

func NewContextProvider(client kubernetes.Interface, imageOpts []imagedataloader.Option) (Context, error) {
	idl, err := imagedataloader.New(client.CoreV1().Secrets(config.KyvernoNamespace()), imageOpts...)
	if err != nil {
		return nil, err
	}
	return &contextProvider{
		client:    client,
		imagedata: idl,
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

func (cp *contextProvider) GetGlobalReference(string) (any, error) {
	return nil, nil
}

func (cp *contextProvider) GetImageData(image string) (*imagedataloader.ImageData, error) {
	// TODO: get image credentials from image verification policies?
	return cp.imagedata.FetchImageData(context.TODO(), image)
}
