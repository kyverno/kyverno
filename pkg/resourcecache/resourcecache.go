package resourcecache

import (
	"fmt"

	"github.com/kyverno/kyverno/pkg/common"
)

// CreateInformers ...
func (resc *resourceCache) CreateInformers(resources ...string) []error {
	var errs []error
	for _, resource := range resources {
		if _, err := resc.CreateGVKInformer(resource); err != nil {
			errs = append(errs, fmt.Errorf("failed to create informer for %s: %v", resource, err))
		}
	}
	return errs
}

// StopResourceInformer - delete the given resource information from ResourceCache and stop watching for the given resource
func (resc *resourceCache) StopResourceInformer(resource string) {
	res, ok := resc.GetGVRCache(resource)
	if ok {
		resc.gvrCache.Remove(resource)
		resc.log.V(4).Info("deleted resource from gvr cache", "name", resource)
		res.StopInformer()
		resc.log.V(4).Info("closed informer for resource", "name", resource)
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

// CreateGVKInformer creates informer for the given gvk
func (resc *resourceCache) CreateGVKInformer(gvk string) (GenericCache, error) {
	gc, ok := resc.GetGVRCache(gvk)
	if ok {
		return gc, nil
	}
	gv, k := common.GetKindFromGVK(gvk)
	apiResource, gvr, err := resc.dclient.DiscoveryClient.FindResource(gv, k)
	if err != nil {
		return nil, fmt.Errorf("cannot find API resource %s", gvk)
	}

	stopCh := make(chan struct{})
	genInformer := resc.dinformer.ForResource(gvr)
	gvrIface := NewGVRCache(gvr, apiResource.Namespaced, stopCh, genInformer)

	resc.gvrCache.Set(gvk, gvrIface)
	resc.dinformer.Start(stopCh)

	if synced := resc.dinformer.WaitForCacheSync(stopCh); !synced[gvr] {
		return nil, fmt.Errorf("informer for %s hasn't synced", gvr)
	}

	return gvrIface, nil
}
