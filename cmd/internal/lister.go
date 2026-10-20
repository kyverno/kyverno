package internal

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// the purpose of this lister is because when we want to resolve registry credentials
// we use an informer based lister. but we may want to resolve secrets from multiple
// namespaces so we will need a lister backed by an informer in each namespace. the alternative
// is a global informer, but that would mean users need to grant kyverno access to all secrets
// in all namespaces which is kind of a big ask.
type multiLister struct {
	listersMap map[string]corev1listers.SecretLister
	corev1listers.SecretListerExpansion
}

func (m *multiLister) Secrets(ns string) corev1listers.SecretNamespaceLister {
	if lister, ok := m.listersMap[ns]; ok {
		return lister.Secrets(ns)
	}
	return emptySecretNamespaceLister{}
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
