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

type configMapInformer struct {
	informer cache.SharedIndexInformer
	lister   corev1listers.ConfigMapLister
}

func NewConfigMapInformer(
	client kubernetes.Interface,
	namespace string,
	name string,
	resyncPeriod time.Duration,
) corev1informers.ConfigMapInformer {
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	options := func(lo *metav1.ListOptions) {
		lo.FieldSelector = fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()
	}
	informer := corev1informers.NewFilteredConfigMapInformer(
		client,
		namespace,
		resyncPeriod,
		indexers,
		options,
	)
	lister := corev1listers.NewConfigMapLister(informer.GetIndexer())
	return &configMapInformer{informer, lister}
}

func (i *configMapInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *configMapInformer) Lister() corev1listers.ConfigMapLister {
	return i.lister
}
