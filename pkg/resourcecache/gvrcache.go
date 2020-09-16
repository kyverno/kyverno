package resourcecache

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// GVRCacheIface - allows operation on a single resource
type GVRCacheIface interface {
	StopInformer()
	IsNamespaced() bool
	GetLister() cache.GenericLister
	GetNamespacedLister(namespace string) cache.GenericNamespaceLister
}

// GVRCache ...
type GVRCache struct {
	// GVR Group Version Resource of a resource
	GVR schema.GroupVersionResource
	// Namespaced - identifies if a resource is namespaced or not
	Namespaced bool
	// stopCh - channel to stop the informer when needed
	stopCh chan struct{}
	// genericInformer - contains instance of informers.GenericInformer for a specific resource
	// which in turn contains Listers() which gives access to cached resources.
	genericInformer informers.GenericInformer
}

// NewGVRCache ...
func NewGVRCache(gvr schema.GroupVersionResource, namespaced bool, stopCh chan struct{}, genericInformer informers.GenericInformer) GVRCacheIface {
	return &GVRCache{GVR: gvr, Namespaced: namespaced, stopCh: stopCh, genericInformer: genericInformer}
}

// StopInformer ...
func (gvrc *GVRCache) StopInformer() {
	close(gvrc.stopCh)
}

// IsNamespaced ...
func (gvrc *GVRCache) IsNamespaced() bool {
	return gvrc.Namespaced
}

// GetLister - get access to Lister() instance of a resource in GVRCache
func (gvrc *GVRCache) GetLister() cache.GenericLister {
	return gvrc.genericInformer.Lister()
}

// GetNamespacedLister - get access to namespaced Lister() instance of a resource in GVRCache
func (gvrc *GVRCache) GetNamespacedLister(namespace string) cache.GenericNamespaceLister {
	return gvrc.genericInformer.Lister().ByNamespace(namespace)
}
