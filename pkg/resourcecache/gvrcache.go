package resourcecache

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamiclister"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

// GenericCache - allows operation on a single resource
type GenericCache interface {
	StopInformer()
	IsNamespaced() bool
	Lister() dynamiclister.Lister
	NamespacedLister(namespace string) dynamiclister.NamespaceLister
	GVR() schema.GroupVersionResource
	GetInformer() cache.SharedIndexInformer
}

type genericCache struct {
	// GVR Group Version Resource of a resource
	gvr schema.GroupVersionResource
	// Namespaced - identifies if a resource is namespaced or not
	namespaced bool
	// stopCh - channel to stop the informer when needed
	stopCh chan struct{}
	// genericInformer - contains instance of informers.GenericInformer for a specific resource
	// which in turn contains Listers() which gives access to cached resources.
	genericInformer informers.GenericInformer
}

// NewGVRCache ...
func NewGVRCache(gvr schema.GroupVersionResource, namespaced bool, stopCh chan struct{}, genericInformer informers.GenericInformer) GenericCache {
	return &genericCache{gvr: gvr, namespaced: namespaced, stopCh: stopCh, genericInformer: genericInformer}
}

// GVR gets GroupVersionResource
func (gc *genericCache) GVR() schema.GroupVersionResource {
	return gc.gvr
}

// StopInformer ...
func (gc *genericCache) StopInformer() {
	close(gc.stopCh)
}

// IsNamespaced ...
func (gc *genericCache) IsNamespaced() bool {
	return gc.namespaced
}

// Lister - get access to Lister() instance of a resource in GVRCache
func (gc *genericCache) Lister() dynamiclister.Lister {
	return dynamiclister.New(gc.genericInformer.Informer().GetIndexer(), gc.GVR())
}

// NamespacedLister - get access to namespaced Lister() instance of a resource in GVRCache
func (gc *genericCache) NamespacedLister(namespace string) dynamiclister.NamespaceLister {
	return dynamiclister.New(gc.genericInformer.Informer().GetIndexer(), gc.GVR()).Namespace(namespace)
}

// GetInformer gets SharedIndexInformer
func (gc *genericCache) GetInformer() cache.SharedIndexInformer {
	return gc.genericInformer.Informer()
}
