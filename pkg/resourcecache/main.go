package resourcecache

import (
	// "fmt"
	// "time"
	"github.com/go-logr/logr"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	// "k8s.io/client-go/informers"
)

type ResourceCacheIface interface {
	RunAllInformers(log logr.Logger)
	CreateResourceInformer(log logr.Logger, resource string) (bool, error)
	StopResourceInformer(log logr.Logger, resource string) bool
	GetGVRCache(resource string) *GVRCache
}

type ResourceCache struct {
	config    *rest.Config
	dclient   *dclient.Client
	dinformer dynamicinformer.DynamicSharedInformerFactory
	match     []string
	exclude   []string

	GVRCacheData map[string]*GVRCache
}

func NewResourceCache(log logr.Logger, config *rest.Config, dclient *dclient.Client, match []string, exclude []string) (ResourceCacheIface, error) {
	logger := log.WithName("resourcecache")
	discoveryIface := dclient.GetDiscoveryCache()
	cacheData := make(map[string]*GVRCache)

	dInformer := dclient.NewDynamicSharedInformerFactory(0)

	resCache := &ResourceCache{config: config, dclient: dclient, GVRCacheData: cacheData, dinformer: dInformer, match: match, exclude: exclude}

	err := udateGVRCache(logger, resCache, discoveryIface)
	if err != nil {
		logger.Error(err, "error in udateGVRCache function")
		return nil, err
	}
	return resCache, nil
}

func udateGVRCache(log logr.Logger, resc *ResourceCache, discoveryIface discovery.CachedDiscoveryInterface) error {
	serverResources, err := discoveryIface.ServerPreferredResources()
	if err != nil {
		return err
	}
	for _, serverResource := range serverResources {
		groupVersion := serverResource.GroupVersion
		for _, resource := range serverResource.APIResources {
			gv, err := schema.ParseGroupVersion(groupVersion)
			if err != nil {
				return err
			}
			mok := resc.matchGVRKey(resource.Name)
			if !mok {
				continue
			}
			eok := resc.excludeGVRKey(resource.Name)
			if eok {
				continue
			}
			_, ok := resc.GVRCacheData[resource.Name]
			if !ok {
				gvrc := &GVRCache{GVR: gv.WithResource(resource.Name), Namespaced: resource.Namespaced}
				resc.GVRCacheData[resource.Name] = gvrc
			}
		}
	}
	return nil
}
