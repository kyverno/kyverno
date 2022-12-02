package resolvers

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type informerBasedResolver struct {
	lister corev1listers.ConfigMapLister
}

func NewInformerBasedResolver(lister corev1listers.ConfigMapLister) (ConfigmapResolver, error) {
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

func NewClientBasedResolver(client kubernetes.Interface) (ConfigmapResolver, error) {
	if client == nil {
		return nil, errors.New("client must not be nil")
	}
	return &clientBasedResolver{client}, nil
}

func (c *clientBasedResolver) Get(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	return c.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
}

type resolverChain []ConfigmapResolver

func NewResolverChain(resolvers ...ConfigmapResolver) (ConfigmapResolver, error) {
	if len(resolvers) == 0 {
		return nil, errors.New("no resolvers")
	}
	for _, resolver := range resolvers {
		if resolver == nil {
			return nil, errors.New("at least one resolver is nil")
		}
	}
	return resolverChain(resolvers), nil
}

func (chain resolverChain) Get(ctx context.Context, namespace, name string) (*corev1.ConfigMap, error) {
	// if CM is not found in informer cache, error will be stored in
	// lastErr variable and resolver chain will try to get CM using
	// Kubernetes client
	var lastErr error
	for _, resolver := range chain {
		cm, err := resolver.Get(ctx, namespace, name)
		if err == nil {
			return cm, nil
		}
		lastErr = err
	}
	return nil, lastErr
}
