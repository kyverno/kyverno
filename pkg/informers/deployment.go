package informers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

type deploymentInformer struct {
	informer cache.SharedIndexInformer
	lister   appsv1listers.DeploymentLister
}

func NewDeploymentInformer(
	client kubernetes.Interface,
	namespace string,
	name string,
	resyncPeriod time.Duration,
) appsv1informers.DeploymentInformer {
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	options := func(lo *metav1.ListOptions) {
		lo.FieldSelector = fields.OneTermEqualSelector(metav1.ObjectNameField, name).String()
	}
	informer := appsv1informers.NewFilteredDeploymentInformer(
		client,
		namespace,
		resyncPeriod,
		indexers,
		options,
	)
	lister := appsv1listers.NewDeploymentLister(informer.GetIndexer())
	return &deploymentInformer{informer, lister}
}

func (i *deploymentInformer) Informer() cache.SharedIndexInformer {
	return i.informer
}

func (i *deploymentInformer) Lister() appsv1listers.DeploymentLister {
	return i.lister
}
