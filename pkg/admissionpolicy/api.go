package admissionpolicy

import (
	"context"

	"github.com/kyverno/kyverno/pkg/clients/dclient"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

type CustomNamespaceLister struct {
	dClient dclient.Interface
}

func (c *CustomNamespaceLister) List(selector labels.Selector) (ret []*corev1.Namespace, err error) {
	namespace, err := c.dClient.GetKubeClient().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	namespaces := make([]*corev1.Namespace, 0, len(namespace.Items))
	for _, ns := range namespace.Items {
		nsCopy := ns
		namespaces = append(namespaces, &nsCopy)
	}
	return namespaces, nil
}

func (c *CustomNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	namespace, err := c.dClient.GetKubeClient().CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return namespace, nil
}

func NewCustomNamespaceLister(dClient dclient.Interface) corev1listers.NamespaceLister {
	return &CustomNamespaceLister{
		dClient: dClient,
	}
}
