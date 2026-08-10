package externalversions

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"

	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
)

// ExtendedSharedInformerFactory wraps the generated SharedInformerFactory and adds
// a dynamic-client fallback so that ForResource works for any GroupVersionResource,
// not only the Kyverno CRD types that are statically registered in the generated switch.
//
// The generated factory covers every Kyverno-owned CRD (ClusterPolicy, PolicyException,
// etc.). For anything else — native Kubernetes resources such as Deployments, or
// third-party CRDs — this wrapper falls back to a dynamicinformer.DynamicSharedInformerFactory
// backed by a dynamic.Interface client, fulfilling the TODO left in generic.go.
type ExtendedSharedInformerFactory struct {
	SharedInformerFactory
	dynFactory dynamicinformer.DynamicSharedInformerFactory
}

// NewExtendedSharedInformerFactory builds an ExtendedSharedInformerFactory.
// It requires both a typed versioned client (for the generated informers) and a
// dynamic client (for the fallback informers).
func NewExtendedSharedInformerFactory(
	client versioned.Interface,
	dynClient dynamic.Interface,
	defaultResync time.Duration,
	options ...SharedInformerOption,
) *ExtendedSharedInformerFactory {
	return &ExtendedSharedInformerFactory{
		SharedInformerFactory: NewSharedInformerFactoryWithOptions(client, defaultResync, options...),
		dynFactory:            dynamicinformer.NewDynamicSharedInformerFactory(dynClient, defaultResync),
	}
}

// ForResource returns a GenericInformer for the given GroupVersionResource.
//
// It first tries the statically generated switch in the embedded SharedInformerFactory,
// which covers all Kyverno CRD types efficiently. If the resource is not found there
// (i.e. it is a native Kubernetes resource or an external CRD), it falls back to the
// dynamic informer factory, satisfying the original TODO in generic.go.
func (f *ExtendedSharedInformerFactory) ForResource(resource schema.GroupVersionResource) (GenericInformer, error) {
	if inf, err := f.SharedInformerFactory.ForResource(resource); err == nil {
		return inf, nil
	}
	dynInf := f.dynFactory.ForResource(resource)
	return &genericInformer{
		informer: dynInf.Informer(),
		resource: resource.GroupResource(),
	}, nil
}

// Start starts both the static informers and the dynamic informers.
func (f *ExtendedSharedInformerFactory) Start(stopCh <-chan struct{}) {
	f.SharedInformerFactory.Start(stopCh)
	f.dynFactory.Start(stopCh)
}

// Shutdown stops both the static informers and the dynamic informers.
func (f *ExtendedSharedInformerFactory) Shutdown() {
	f.SharedInformerFactory.Shutdown()
	f.dynFactory.Shutdown()
}
