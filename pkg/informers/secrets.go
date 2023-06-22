package informers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

type secretInformer struct {
	informer cache.SharedIndexInformer
	lister   corev1listers.SecretLister
}

func NewSecretInformer(
	client kubernetes.Interface,
	namespace string,
	name string,
	resyncPeriod time.Duration,
) corev1informers.SecretInformer {
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	options := func(lo *metav1.ListOptions) {
		lo.FieldSelector = fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()
	}
	informer := corev1informers.NewFilteredSecretInformer(
		client,
		namespace,
		resyncPeriod,
		indexers,
		options,
	)
	lister := corev1listers.NewSecretLister(informer.GetIndexer())
	return &secretInformer{informer, lister}
}

func (i *secretInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *secretInformer) Lister() corev1listers.SecretLister {
	return i.lister
}
