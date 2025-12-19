package cluster

import (
	"context"

	"github.com/kyverno/kyverno/ext/resource/convert"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	v1 "k8s.io/client-go/listers/core/v1"
)

type secretLister struct {
	client    dclient.Interface
	namespace string
}

func (s *secretLister) List(selector labels.Selector) (ret []*corev1.Secret, err error) {
	l, err := metav1.ParseToLabelSelector(selector.String())
	if err != nil {
		return nil, err
	}

	list, err := s.client.ListResource(context.TODO(), "v1", "Secret", s.namespace, l)
	if err != nil {
		return nil, err
	}

	results := make([]*corev1.Secret, 0, len(list.Items))
	for _, s := range list.Items {
		secret, err := convert.To[corev1.Secret](s)
		if err != nil {
			continue
		}

		results = append(results, secret)
	}

	return results, nil
}

func (s *secretLister) Get(name string) (*corev1.Secret, error) {
	object, err := s.client.GetResource(context.TODO(), "v1", "Secret", s.namespace, name)
	if err != nil {
		return nil, err
	}

	secret, err := convert.To[corev1.Secret](*object)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func NewSecretLister(client dclient.Interface, ns string) v1.SecretNamespaceLister {
	return &secretLister{client, ns}
}
