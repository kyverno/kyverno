package resourcecache

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type GVRCacheIface interface {
	StopInformer()
	IsNamespaced() bool
	GetLister() cache.GenericLister
	GetNamespacedLister(namespace string) cache.GenericNamespaceLister
}

type GVRCache struct {
	GVR schema.GroupVersionResource
	Namespaced bool
	stopCh chan struct{}
	genericInformer informers.GenericInformer
}

func NewGVRCache(gvr schema.GroupVersionResource, namespaced bool, stopCh chan struct{}, genericInformer informers.GenericInformer) GVRCacheIface{
	return &GVRCache{GVR: gvr, Namespaced: namespaced, stopCh: stopCh, genericInformer: genericInformer}
}

func (gvrc *GVRCache) StopInformer() {
	close(gvrc.stopCh)
}

func (gvrc *GVRCache) IsNamespaced() bool{
	return gvrc.Namespaced
}

func (gvrc *GVRCache) GetLister() cache.GenericLister {
	return gvrc.genericInformer.Lister()
}

func (gvrc *GVRCache) GetNamespacedLister(namespace string) cache.GenericNamespaceLister {
	return gvrc.genericInformer.Lister().ByNamespace(namespace)
}