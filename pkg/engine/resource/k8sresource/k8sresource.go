package k8sresource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/resource/cache"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	k8scache "k8s.io/client-go/tools/cache"
)

const resyncPeriod = 15 * time.Second

type ResourceLoader struct {
	logger logr.Logger
	client dynamic.Interface
	cache  cache.Cache
}

type resourceEntry struct {
	sync.Mutex
	logger          logr.Logger
	lister          k8scache.GenericNamespaceLister
	watchErrHandler *WatchErrorHandler
	lastUpdated     time.Time
	cancel          context.CancelFunc
}

func (re *resourceEntry) Get() (interface{}, error) {
	re.Lock()
	defer re.Unlock()

	re.logger.V(4).Info("fetching data from resource cache entry")
	re.lastUpdated = time.Now()
	if re.watchErrHandler.Error() != nil {
		re.logger.Error(re.watchErrHandler.Error(), "failed to fetch data from entry")
		return nil, re.watchErrHandler.Error()
	}
	obj, err := re.lister.List(labels.Everything())
	if err != nil {
		re.logger.Error(err, "failed to fetch data from entry")
		return nil, err
	}
	re.logger.V(6).Info("cache entry data", "total fetched:", len(obj))
	for _, o := range obj {
		metadata, err := meta.Accessor(o)
		if err != nil {
			continue
		}
		re.logger.V(6).Info("cache entry data", "name", metadata.GetName(), "namespace", metadata.GetNamespace())
	}
	return obj, nil
}

func (re *resourceEntry) LastUpdated() time.Time {
	return re.lastUpdated
}

func (re *resourceEntry) Stop() {
	re.cancel()
}

func New(logger logr.Logger, dclient dynamic.Interface, c cache.Cache) *ResourceLoader {
	logger = logger.WithName("k8s resource loader")
	return &ResourceLoader{
		logger: logger,
		client: dclient,
		cache:  c,
	}
}

func (r *ResourceLoader) SetEntries(entries ...*v2alpha1.CachedContextEntry) {
	for _, entry := range entries {
		if entry.Spec.K8sResource == nil {
			continue
		}
		r.SetEntry(entry)
	}
}

func (r *ResourceLoader) SetEntry(entry *v2alpha1.CachedContextEntry) {
	if entry.Spec.K8sResource == nil {
		return
	}
	rc := entry.Spec.K8sResource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Resource,
	}
	key := getKeyForResourceEntry(resource, rc.Namespace, entry.Name)
	ent, err := r.createGenericListerForResource(resource, rc.Namespace)
	if err != nil {
		ent = cache.NewInvalidEntry(err)
	}
	ok := r.cache.Set(key, ent)
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		r.logger.Error(err, "")
		return
	}
	r.logger.V(4).Info("successfully created cache entry", "key", key, "entry", ent)
}

func (r *ResourceLoader) UpdateEntry(oldEntry, newEntry *v2alpha1.CachedContextEntry) {
	oldrc := oldEntry.Spec.K8sResource.DeepCopy()
	newrc := newEntry.Spec.K8sResource.DeepCopy()

	oldresource := schema.GroupVersionResource{
		Group:    oldrc.Group,
		Version:  oldrc.Version,
		Resource: oldrc.Resource,
	}
	newresource := schema.GroupVersionResource{
		Group:    newrc.Group,
		Version:  newrc.Version,
		Resource: newrc.Resource,
	}
	oldkey := getKeyForResourceEntry(oldresource, oldrc.Namespace, oldEntry.Name)
	newkey := getKeyForResourceEntry(newresource, newrc.Namespace, newEntry.Name)

	r.SetEntry(newEntry)
	if oldkey == newkey {
		return
	}

	r.Delete(oldEntry)
}

func (r *ResourceLoader) Get(rc *kyvernov1.ResourceCache) (interface{}, error) {
	if rc.K8sResource == nil {
		return nil, fmt.Errorf("resource not found")
	}
	resource := schema.GroupVersionResource{
		Group:    rc.K8sResource.Group,
		Version:  rc.K8sResource.Version,
		Resource: rc.K8sResource.Resource,
	}
	key := getKeyForResourceEntry(resource, rc.K8sResource.Namespace, "")
	entry, ok := r.cache.Get(key)
	if !ok {
		err := fmt.Errorf("failed to fetch entry key=%s", key)
		r.logger.Error(err, "")
		return nil, err
	}
	r.logger.V(4).Info("successfully fetched cache entry", "key", key, "entry", entry)
	return entry.Get()
}

func (r *ResourceLoader) Delete(entry *v2alpha1.CachedContextEntry) {
	if entry.Spec.K8sResource == nil {
		return
	}
	rc := entry.Spec.K8sResource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Resource,
	}
	key := getKeyForResourceEntry(resource, rc.Namespace, entry.Name)
	r.cache.Delete(key)

	r.logger.V(4).Info("successfully deleted cache entry")
}

func (r *ResourceLoader) createGenericListerForResource(resource schema.GroupVersionResource, namespace string) (cache.ResourceEntry, error) {
	informer := dynamicinformer.NewFilteredDynamicInformer(r.client, resource, namespace, resyncPeriod, k8scache.Indexers{k8scache.NamespaceIndex: k8scache.MetaNamespaceIndexFunc}, nil)
	watchErrHandler := NewWatchErrorHandler(r.logger, resource, namespace)
	err := informer.Informer().SetWatchErrorHandler(watchErrHandler.WatchErrorHandlerFunction())
	if err != nil {
		r.logger.Error(err, "failed to add watch error handler")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.TODO())
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
	return &resourceEntry{lister: lister, logger: r.logger.WithName("k8s resource entry"), watchErrHandler: watchErrHandler, lastUpdated: time.Now(), cancel: cancel}, nil
}

func getKeyForResourceEntry(resource schema.GroupVersionResource, namespace, entryname string) string {
	return strings.Join([]string{"Resource= ", resource.String(), ", Namespace=", namespace, "EntryName=", entryname}, "")
}
