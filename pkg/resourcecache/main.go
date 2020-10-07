package resourcecache

import (
	// "fmt"
	// "time"
	"github.com/go-logr/logr"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
)

// ResourceCacheIface - allows the creation, deletion and saving the resource informers as a cache
type ResourceCacheIface interface {
	RunAllInformers(log logr.Logger)
	CreateResourceInformer(log logr.Logger, resource string) (bool, error)
	StopResourceInformer(log logr.Logger, resource string) bool
	GetGVRCache(resource string) *GVRCache
}

// ResourceCache ...
type ResourceCache struct {
	dinformer dynamicinformer.DynamicSharedInformerFactory
	// match - matches the resources for which the informers needs to be created.
	// if the matches contains any resource name then the informers are created for only that resource
	// else informers are created for all the server supported resources
	match []string

	// excludes the creation of informers for a specific resources
	// if a specific resource is available in both match and exclude then exclude overrides it
	exclude []string

	// GVRCacheData - stores the informers and lister object for a resource.
	// it uses resource name as key (For ex :-  namespaces for Namespace, pods for Pod, clusterpolicies for ClusterPolicy etc)
	// GVRCache stores GVR (Group Version Resource) for the resource, Informer() instance and Lister() instance for that resource.
	GVRCacheData map[string]*GVRCache
}

// NewResourceCache - initializes the ResourceCache where it initially stores the GVR and Namespaced codition for the allowed resources in GVRCacheData
func NewResourceCache(log logr.Logger, config *rest.Config, dclient *dclient.Client, match []string, exclude []string) (ResourceCacheIface, error) {
	logger := log.WithName("resourcecache")
	discoveryIface := dclient.GetDiscoveryCache()
	cacheData := make(map[string]*GVRCache)

	dInformer := dclient.NewDynamicSharedInformerFactory(0)

	resCache := &ResourceCache{GVRCacheData: cacheData, dinformer: dInformer, match: match, exclude: exclude}

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
