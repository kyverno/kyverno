package k8sresource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/resourcecache/cache"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	k8scache "k8s.io/client-go/tools/cache"
)

type ResourceLoader struct {
	logger logr.Logger
	client dynamic.Interface
	cache  cache.Cache
}

type resourceEntry struct {
	logger logr.Logger
	lister k8scache.GenericNamespaceLister
}

func (re *resourceEntry) Get() (interface{}, error) {
	obj, err := re.lister.List(labels.Everything())
	if err != nil {
		re.logger.Error(err, "failed to fetch data from entry")
		return nil, err
	}
	re.logger.V(6).Info("cache entry data", "len", len(obj), "data", obj)
	return obj, nil
}

func New(logger logr.Logger, dclient dynamic.Interface, c cache.Cache) *ResourceLoader {
	logger = logger.WithName("k8s resource loader")
	return &ResourceLoader{
		logger: logger,
		client: dclient,
		cache:  c,
	}
}

func (r *ResourceLoader) AddEntries(entries ...*v2alpha1.CachedContextEntry) error {
	for _, entry := range entries {
		if entry.Spec.Resource == nil {
			continue
		}
		err := r.AddEntry(entry)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *ResourceLoader) AddEntry(entry *v2alpha1.CachedContextEntry) error {
	if entry.Spec.Resource == nil {
		err := fmt.Errorf("Invalid object provided")
		r.logger.Error(err, "")
		return err
	}
	rc := entry.Spec.Resource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Kind,
	}
	key := getKeyForResourceEntry(resource, rc.Namespace)
	ent, err := r.createGenericListerForResource(resource, rc.Namespace)
	if err != nil {
		return err
	}
	r.logger.V(2).Info("key", key, "entry", ent)
	ok := r.cache.Add(key, ent)
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		r.logger.Error(err, "")
		return err
	}
	r.logger.V(2).Info("successfully created cache entry")
	return nil
}

func (r *ResourceLoader) Get(rc *kyvernov1.ResourceCache) (interface{}, error) {
	if rc.Resource == nil {
		return nil, fmt.Errorf("resource not found")
	}
	resource := schema.GroupVersionResource{
		Group:    rc.Resource.Group,
		Version:  rc.Resource.Version,
		Resource: rc.Resource.Kind,
	}
	key := getKeyForResourceEntry(resource, rc.Resource.Namespace)
	entry, ok := r.cache.Get(key)
	if !ok {
		err := fmt.Errorf("failed to create fetch entry key=%s", key)
		r.logger.Error(err, "")
		return nil, err
	}
	r.logger.V(2).Info("successfully fetched cache entry")
	return entry.Get()
}

func (r *ResourceLoader) Delete(entry *v2alpha1.CachedContextEntry) error {
	if entry.Spec.Resource == nil {
		return fmt.Errorf("invalid object provided")
	}
	rc := entry.Spec.Resource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Kind,
	}
	key := getKeyForResourceEntry(resource, rc.Namespace)
	ok := r.cache.Delete(key)
	if !ok {
		err := fmt.Errorf("failed to delete k8s object entry")
		r.logger.Error(err, "")
		return err
	}
	r.logger.V(2).Info("successfully deleted cache entry")
	return nil
}

func (r *ResourceLoader) createGenericListerForResource(resource schema.GroupVersionResource, namespace string) (*cache.CacheEntry, error) {
	informer := dynamicinformer.NewFilteredDynamicInformer(r.client, resource, namespace, 5*time.Second, k8scache.Indexers{k8scache.NamespaceIndex: k8scache.MetaNamespaceIndexFunc}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go informer.Informer().Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		cancel()
		err := errors.New("resource informer cache failed to sync")
		r.logger.Error(err, "")
		return nil, err
	}

	var lister k8scache.GenericNamespaceLister
	if len(namespace) == 0 {
		lister = informer.Lister()
	} else {
		lister = informer.Lister().ByNamespace(namespace)
	}
	return &cache.CacheEntry{
		Entry: &resourceEntry{lister: lister, logger: r.logger.WithName("k8s resource entry")},
		Stop:  cancel,
	}, nil
}

func getKeyForResourceEntry(resource schema.GroupVersionResource, namespace string) string {
	return strings.Join([]string{"Resource= ", resource.String(), ", Namespace=", namespace}, "")
}
