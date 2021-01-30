package resourcecache

import (
	"github.com/go-logr/logr"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	cmap "github.com/orcaman/concurrent-map"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// ResourceCache - allows the creation, deletion and saving the resource informers as a cache
type ResourceCache interface {
	CreateInformers(resources ...string) []error
	CreateResourceInformer(resource string) (GenericCache, error)
	StopResourceInformer(resource string)
	GetGVRCache(resource string) (GenericCache, bool)
}

type resourceCache struct {
	dclient *dclient.Client

	dinformer dynamicinformer.DynamicSharedInformerFactory

	// gvrCache - stores the manipulate factory for a resource
	// it uses resource name as key (i.e., namespaces for Namespace, pods for Pod, clusterpolicies for ClusterPolicy, etc)
	gvrCache cmap.ConcurrentMap

	log logr.Logger
}

// NewResourceCache - initializes the ResourceCache
func NewResourceCache(dclient *dclient.Client, dInformer dynamicinformer.DynamicSharedInformerFactory, logger logr.Logger) (ResourceCache, error) {
	rCache := &resourceCache{
		dclient:   dclient,
		gvrCache:  cmap.New(),
		dinformer: dInformer,
		log:       logger,
	}

	if _, err := rCache.CreateResourceInformer("ConfigMap"); err != nil {
		return nil, err
	}

	return rCache, nil
}
