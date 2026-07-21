package registryclient

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8scorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type secretListerAdapter struct {
	client    k8scorev1.SecretInterface
	namespace string
}

type secretNamespaceListerAdapter struct {
	client    k8scorev1.SecretInterface
	namespace string
	allowed   bool
}

func SecretListerFromInterface(client k8scorev1.SecretInterface, namespace string) corev1listers.SecretLister {
	if client == nil {
		return nil
	}
	return &secretListerAdapter{client: client, namespace: namespace}
}

func (s *secretListerAdapter) List(selector labels.Selector) ([]*corev1.Secret, error) {
	return s.Secrets(s.namespace).List(selector)
}

func (s *secretListerAdapter) Secrets(namespace string) corev1listers.SecretNamespaceLister {
	return &secretNamespaceListerAdapter{
		client:    s.client,
		namespace: namespace,
		allowed:   namespace == s.namespace,
	}
}

func (s *secretNamespaceListerAdapter) List(selector labels.Selector) ([]*corev1.Secret, error) {
	if s.client == nil || !s.allowed {
		return nil, nil
	}
	list, err := s.client.List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, err
	}
	result := make([]*corev1.Secret, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, &list.Items[i])
	}
	return result, nil
}

func (s *secretNamespaceListerAdapter) Get(name string) (*corev1.Secret, error) {
	if s.client == nil || !s.allowed {
		return nil, apierrors.NewNotFound(schema.GroupResource{Group: "", Resource: "secrets"}, name)
	}
	return s.client.Get(context.TODO(), name, metav1.GetOptions{})
}
