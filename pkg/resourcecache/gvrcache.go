package resourcecache

import (
	"github.com/go-logr/logr"

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
	AddCacheWorker(workerUID string) cacheWorker
	RemoveCacheWorker(workerUID string) cacheWorker
	GetCacheWorker(workerUID string) (cacheWorker, bool)
	GetCacheWorkers() map[string]cacheWorker
}

type cacheWorker struct{}

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
	// cacheWorkers contains of workers depending upon this cache keyed on a worker uid
	// used to indicate whether the cache is no longer in use and can be removed
	cacheWorkers map[string]cacheWorker

	log logr.Logger
}

// NewGVRCache ...
func NewGVRCache(gvr schema.GroupVersionResource, namespaced bool, stopCh chan struct{}, genericInformer informers.GenericInformer, logger logr.Logger) GenericCache {
	return &genericCache{
		gvr:             gvr,
		namespaced:      namespaced,
		stopCh:          stopCh,
		genericInformer: genericInformer,
		cacheWorkers:    make(map[string]cacheWorker),
		log:             logger,
	}
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

// AddCacheWorker adds the worker lock to the cache
func (gc *genericCache) AddCacheWorker(workerUID string) cacheWorker {
	if _, ok := gc.cacheWorkers[workerUID]; !ok {
		gc.cacheWorkers[workerUID] = cacheWorker{}
	}
	return gc.cacheWorkers[workerUID]
}

// RemoveCacheWorker removes the worker lock from the cache
func (gc *genericCache) RemoveCacheWorker(workerUID string) cacheWorker {
	cw := gc.cacheWorkers[workerUID]
	if _, ok := gc.cacheWorkers[workerUID]; ok {
		delete(gc.cacheWorkers, workerUID)
	}
	return cw
}

// GetCacheWorker gets the cacheWorker for the cache if it exists, returning false if not
func (gc *genericCache) GetCacheWorker(workerUID string) (cacheWorker, bool) {
	worker, ok := gc.cacheWorkers[workerUID]
	return worker, ok
}

// GetCacheWorkers gets the cacheworkers
func (gc *genericCache) GetCacheWorkers() map[string]cacheWorker {
	return gc.cacheWorkers
}
