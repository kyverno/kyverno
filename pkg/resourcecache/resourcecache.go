package resourcecache

import (
	"github.com/go-logr/logr"
)

// RunAllInformers - run the informers for the GVR of all the resources available in GVRCacheData
func (resc *ResourceCache) RunAllInformers(log logr.Logger) {
	for key := range resc.GVRCacheData {
		resc.CreateResourceInformer(log, key)
		log.V(4).Info("created informer for resource", "name", key)
	}
}

// CreateResourceInformer - check the availability of the given resource in ResourceCache.
// if available then create an informer for that GVR and store that GenericInformer instance in the cache and start watching for that resource
func (resc *ResourceCache) CreateResourceInformer(log logr.Logger, resource string) (bool, error) {
	res, ok := resc.GVRCacheData[resource]
	if ok {
		stopCh := make(chan struct{})
		res.stopCh = stopCh
		genInformer := resc.dinformer.ForResource(res.GVR)
		res.genericInformer = genInformer
		go startWatching(stopCh, genInformer.Informer())
	}
	return true, nil
}

// StopResourceInformer - delete the given resource information from ResourceCache and stop watching for the given resource
func (resc *ResourceCache) StopResourceInformer(log logr.Logger, resource string) bool {
	res, ok := resc.GVRCacheData[resource]
	if ok {
		delete(resc.GVRCacheData, resource)
		log.V(4).Info("deleted resource from gvr cache", "name", resource)
		res.StopInformer()
		log.V(4).Info("closed informer for resource", "name", resource)
	}
	return false
}

// GetGVRCache - get the GVRCache for a given resource if available
func (resc *ResourceCache) GetGVRCache(resource string) *GVRCache {
	res, ok := resc.GVRCacheData[resource]
	if ok {
		return res
	}
	return nil
}
