package resolvers

import (
	"context"
	"errors"

	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type informerBasedResolver struct {
	lister corev1listers.ConfigMapLister
}

func NewInformerBasedResolver(lister corev1listers.ConfigMapLister) (engineapi.ConfigmapResolver, error) {
	if lister == nil {
		return nil, errors.New("lister must not be nil")
	}
	return &informerBasedResolver{lister}, nil
}

func (i *informerBasedResolver) Get(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	return i.lister.ConfigMaps(namespace).Get(name)
}

type clientBasedResolver struct {
	kubeClient kubernetes.Interface
}

func NewClientBasedResolver(client kubernetes.Interface) (engineapi.ConfigmapResolver, error) {
	if client == nil {
		return nil, errors.New("client must not be nil")
	}
	return &clientBasedResolver{client}, nil
}

func (c *clientBasedResolver) Get(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	return c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}
