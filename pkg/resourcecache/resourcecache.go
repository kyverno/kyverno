package resourcecache

import (
	"github.com/go-logr/logr"
)

func (resc *ResourceCache) RunAllInformers(log logr.Logger) {
	for key, _ := range resc.GVRCacheData {
		mok := resc.matchGVRKey(key)
		if !mok {
			continue
		}
		eok := resc.excludeGVRKey(key)
		if eok {
			continue
		}
		resc.CreateResourceInformer(log, key)
		log.V(4).Info("created informer for resource", "name", key)
	}
}

func (resc *ResourceCache) CreateResourceInformer(log logr.Logger, resource string) (bool, error) {
	mok := resc.matchGVRKey(resource)
	if !mok {
		return false, nil
	}
	eok := resc.excludeGVRKey(resource)
	if eok {
		return false, nil
	}
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

func (resc *ResourceCache) GetGVRCache(resource string) *GVRCache {
	res, ok := resc.GVRCacheData[resource]
	if ok {
		return res
	}
	return nil
}
