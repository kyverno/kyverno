package informers

import (
	"reflect"
	"time"

	versioned "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// dynamicInformerType is the key under which the aggregated sync state of all
// dynamic fallback informers is reported by WaitForCacheSync. Every dynamic informer
// holds *unstructured.Unstructured objects.
var dynamicInformerType = reflect.TypeOf(&unstructured.Unstructured{})

// Option configures an ExtendedSharedInformerFactory. Options are applied to both
// the typed factory and the dynamic fallback factory so that filtering is consistent.
type Option func(*options)

type options struct {
	namespace string
	tweak     func(*metav1.ListOptions)
}

// WithNamespace limits both the typed and the dynamic informers to a single namespace.
func WithNamespace(namespace string) Option {
	return func(o *options) { o.namespace = namespace }
}

// WithTweakListOptions applies a list-option tweak (e.g. label selector) to both
// the typed and the dynamic informers.
func WithTweakListOptions(tweak func(*metav1.ListOptions)) Option {
	return func(o *options) { o.tweak = tweak }
}

// ExtendedSharedInformerFactory wraps the generated SharedInformerFactory and adds
// a dynamic-client fallback so that ForResource works for any GroupVersionResource,
// not only the Kyverno CRD types that are statically registered in the generated switch.
//
// The generated factory covers every Kyverno-owned CRD (ClusterPolicy, PolicyException,
// etc.). For anything else — native Kubernetes resources such as Deployments, or
// third-party CRDs — this wrapper falls back to a dynamicinformer.DynamicSharedInformerFactory
// backed by a dynamic.Interface client, fulfilling the TODO left in generic.go.
//
// It lives outside pkg/client because that directory is regenerated (and wiped) by codegen.
type ExtendedSharedInformerFactory struct {
	kyvernoinformer.SharedInformerFactory
	dynFactory dynamicinformer.DynamicSharedInformerFactory
}

// NewExtendedSharedInformerFactory builds an ExtendedSharedInformerFactory.
// It requires both a typed versioned client (for the generated informers) and a
// dynamic client (for the fallback informers).
func NewExtendedSharedInformerFactory(
	client versioned.Interface,
	dynClient dynamic.Interface,
	defaultResync time.Duration,
	opts ...Option,
) *ExtendedSharedInformerFactory {
	cfg := &options{namespace: metav1.NamespaceAll}
	for _, opt := range opts {
		opt(cfg)
	}
	var typedOpts []kyvernoinformer.SharedInformerOption
	if cfg.namespace != metav1.NamespaceAll {
		typedOpts = append(typedOpts, kyvernoinformer.WithNamespace(cfg.namespace))
	}
	if cfg.tweak != nil {
		typedOpts = append(typedOpts, kyvernoinformer.WithTweakListOptions(cfg.tweak))
	}
	return &ExtendedSharedInformerFactory{
		SharedInformerFactory: kyvernoinformer.NewSharedInformerFactoryWithOptions(client, defaultResync, typedOpts...),
		dynFactory:            dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynClient, defaultResync, cfg.namespace, cfg.tweak),
	}
}

// ForResource returns a GenericInformer for the given GroupVersionResource.
//
// It first tries the statically generated switch in the embedded SharedInformerFactory,
// which covers all Kyverno CRD types efficiently. If the resource is not found there
// (i.e. it is a native Kubernetes resource or an external CRD), it falls back to the
// dynamic informer factory, satisfying the original TODO in generic.go.
func (f *ExtendedSharedInformerFactory) ForResource(resource schema.GroupVersionResource) (kyvernoinformer.GenericInformer, error) {
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

// WaitForCacheSync waits for the caches of both the static and the dynamic informers.
//
// The generated return type is keyed by reflect.Type, which cannot represent a
// GroupVersionResource, so the sync state of all dynamic informers is aggregated under a
// single *unstructured.Unstructured key: it is true only if every started dynamic
// informer synced. The key is absent when no dynamic informer was started.
func (f *ExtendedSharedInformerFactory) WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool {
	res := f.SharedInformerFactory.WaitForCacheSync(stopCh)
	if res == nil {
		res = map[reflect.Type]bool{}
	}
	dyn := f.dynFactory.WaitForCacheSync(stopCh)
	if len(dyn) == 0 {
		return res
	}
	synced := true
	for _, ok := range dyn {
		synced = synced && ok
	}
	res[dynamicInformerType] = synced
	return res
}

// Shutdown stops both the static informers and the dynamic informers.
func (f *ExtendedSharedInformerFactory) Shutdown() {
	f.SharedInformerFactory.Shutdown()
	f.dynFactory.Shutdown()
}

// genericInformer mirrors the unexported type of the same name in the generated package.
type genericInformer struct {
	informer cache.SharedIndexInformer
	resource schema.GroupResource
}

// Informer returns the SharedIndexInformer.
func (f *genericInformer) Informer() cache.SharedIndexInformer {
	return f.informer
}

// Lister returns the GenericLister.
func (f *genericInformer) Lister() cache.GenericLister {
	return cache.NewGenericLister(f.Informer().GetIndexer(), f.resource)
}
