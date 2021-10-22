package resourcecache

import (
	"context"
	"fmt"

	"github.com/kyverno/kyverno/pkg/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// CreateInformers ...
func (resc *resourceCache) CreateInformers(resources ...string) []error {
	var errs []error
	for _, resource := range resources {
		if _, err := resc.CreateGVKInformer(resource, rescacheWorkerUID); err != nil {
			errs = append(errs, fmt.Errorf("failed to create informer for %s: %v", resource, err))
		}
	}
	return errs
}

// StopResourceInformer - delete the given resource information from ResourceCache and stop watching for the given resource
func (resc *resourceCache) StopResourceInformer(resource string, workerUID string) {
	res, ok := resc.GetGVRCache(resource)
	if ok {
		res.RemoveCacheWorker(workerUID)
		resc.log.V(4).Info("Stopping cache informer", "resource", resource, "workerUID", workerUID, "cache", res, "workerCount", len(res.GetCacheWorkers()))
		if len(res.GetCacheWorkers()) == 0 {
			resc.gvrCache.Remove(resource)
			resc.log.V(4).Info("Deleted resource from gvr cache", "name", resource)
			res.StopInformer()
			resc.log.V(4).Info("Closed informer for resource", "name", resource)
		}
	}
}

// GetGVRCache - get the GVRCache for a given resource if available
func (resc *resourceCache) GetGVRCache(resource string) (GenericCache, bool) {
	res, ok := resc.gvrCache.Get(resource)
	if ok {
		return res.(*genericCache), true
	}

	return nil, false
}

// GetGVRCachesForWorker - get the GVRCaches locked for the worker
func (resc *resourceCache) GetGVRCachesForWorker(workerUID string) map[string]GenericCache {
	caches := make(map[string]GenericCache)

	resCaches := resc.gvrCache.Items()
	for key, item := range resCaches {
		if cache, ok := item.(*genericCache); ok {
			if _, ok := cache.GetCacheWorker(workerUID); ok {
				caches[key] = cache
			}
		}
	}
	return caches
}

// CreateGVKInformer creates informer for the given gvk
func (resc *resourceCache) CreateGVKInformer(gvk string, workerUID string) (GenericCache, error) {
	gv, k := common.GetKindFromGVK(gvk)
	apiResource, gvr, err := resc.dclient.DiscoveryClient.FindResource(gv, k)
	if err != nil {
		return nil, fmt.Errorf("cannot find API resource %s", gvk)
	}

	return resc.CreateGVRInformer(gvr, apiResource.Namespaced, gvk, workerUID)
}

// CreateGVRInformer creates informer for the given gvr
func (resc *resourceCache) CreateGVRInformer(gvr schema.GroupVersionResource, namespaced bool, cacheKey string, workerUID string) (GenericCache, error) {
	gc, ok := resc.GetGVRCache(cacheKey)
	if ok {
		gc.AddCacheWorker(workerUID)
		return gc, nil
	}

	if _, err := resc.dclient.GetDynamicInterface().Resource(gvr).List(context.TODO(), v1.ListOptions{}); err != nil {
		resc.log.V(4).Info("Error listing resource before creating cache", "error", err)
		return nil, err
	}

	stopCh := make(chan struct{})
	genInformer := resc.dinformer.ForResource(gvr)
	gvrIface := NewGVRCache(gvr, namespaced, stopCh, genInformer, resc.log)

	resc.gvrCache.Set(cacheKey, gvrIface)
	resc.dinformer.Start(stopCh)

	if synced := resc.dinformer.WaitForCacheSync(stopCh); !synced[gvr] {
		return nil, fmt.Errorf("informer for %s hasn't synced", gvr)
	}

	gvrIface.AddCacheWorker(workerUID)
	return gvrIface, nil
}
