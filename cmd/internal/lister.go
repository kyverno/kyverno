package internal

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// the purpose of this lister is because when we want to resolve registry credentials
// we use an informer based lister. but we may want to resolve secrets from multiple
// namespaces so we will need a lister backed by an informer in each namespace. the alternative
// is a global informer, but that would mean users need to grant kyverno access to all secrets
// in all namespaces which is kind of a big ask.
type multiLister struct {
	client     kubernetes.Interface
	listersMap map[string]corev1listers.SecretLister
	corev1listers.SecretListerExpansion
}

func (m *multiLister) Secrets(ns string) corev1listers.SecretNamespaceLister {
	if lister, ok := m.listersMap[ns]; ok {
		return lister.Secrets(ns)
	}
	// Namespaces not covered by informers (e.g. a Pod's own namespace where
	// imagePullSecrets live) fall back to a direct API lookup. Kyverno's
	// default RBAC grants get/list/watch secrets cluster-wide, so this is
	// safe without requiring a global informer (which would be a bigger
	// permission ask just for the rare case of per-Pod imagePullSecrets).
	return &directSecretNamespaceLister{
		client:    m.client,
		namespace: ns,
	}
}

func (m *multiLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	ret := []*corev1.Secret{}
	for _, lister := range m.listersMap {
		listerSecrets, err := lister.List(selector)
		if err != nil {
			return nil, err
		}
		ret = append(ret, listerSecrets...)
	}
	return ret, nil
}

type emptySecretNamespaceLister struct{}

func (emptySecretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	_ = selector
	return nil, nil
}

func (emptySecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "secrets"}, name)
}

// directSecretNamespaceLister uses the Kubernetes API directly to look up
// secrets in a namespace that is not covered by any informer-backed lister.
type directSecretNamespaceLister struct {
	client    kubernetes.Interface
	namespace string
}

func (d *directSecretNamespaceLister) List(selector labels.Selector) ([]*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	list, err := d.client.CoreV1().Secrets(d.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, err
	}
	secrets := make([]*corev1.Secret, len(list.Items))
	for i := range list.Items {
		secrets[i] = &list.Items[i]
	}
	return secrets, nil
}

func (d *directSecretNamespaceLister) Get(name string) (*corev1.Secret, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return d.client.CoreV1().Secrets(d.namespace).Get(ctx, name, metav1.GetOptions{})
}
