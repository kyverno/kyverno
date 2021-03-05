package resourcecache

import (
	"fmt"

	"github.com/go-logr/logr"
	dclient "github.com/kyverno/kyverno/pkg/dclient"
	cmap "github.com/orcaman/concurrent-map"
	"k8s.io/client-go/dynamic/dynamicinformer"
)

// ResourceCache - allows the creation, deletion and saving the resource informers as a cache
type ResourceCache interface {
	CreateInformers(resources ...string) []error
	CreateGVKInformer(kind string) (GenericCache, error)
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

var KyvernoDefaultInformer = []string{"ConfigMap", "Secret", "Deployment", "MutatingWebhookConfiguration", "ValidatingWebhookConfiguration"}

// NewResourceCache - initializes the ResourceCache
func NewResourceCache(dclient *dclient.Client, dInformer dynamicinformer.DynamicSharedInformerFactory, logger logr.Logger) (ResourceCache, error) {
	rCache := &resourceCache{
		dclient:   dclient,
		gvrCache:  cmap.New(),
		dinformer: dInformer,
		log:       logger,
	}

	errs := rCache.CreateInformers(KyvernoDefaultInformer...)
	if len(errs) != 0 {
		return rCache, fmt.Errorf("failed to register default informers %v", errs)
	}

	return rCache, nil
}
