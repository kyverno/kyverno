package k8sresource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
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
	store  store.Store
}

type resourceEntry struct {
	sync.Mutex
	logger          logr.Logger
	lister          k8scache.GenericNamespaceLister
	watchErrHandler *WatchErrorHandler
	cancel          context.CancelFunc
}

func (re *resourceEntry) Get() (interface{}, error) {
	re.Lock()
	defer re.Unlock()

	re.logger.V(4).Info("fetching data from resource cache entry")
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

func (re *resourceEntry) Stop() {
	re.cancel()
}

func New(logger logr.Logger, dclient dynamic.Interface, c store.Store) *ResourceLoader {
	logger = logger.WithName("k8s resource loader")
	return &ResourceLoader{
		logger: logger,
		client: dclient,
		store:  c,
	}
}

func (r *ResourceLoader) SetEntries(entries ...*v2alpha1.GlobalContextEntry) {
	for _, entry := range entries {
		if entry.Spec.K8sResource == nil {
			continue
		}
		r.SetEntry(entry)
	}
}

func (r *ResourceLoader) SetEntry(entry *v2alpha1.GlobalContextEntry) {
	if entry.Spec.K8sResource == nil {
		return
	}
	rc := entry.Spec.K8sResource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Resource,
	}
	key := entry.Name
	ent, err := r.createGenericListerForResource(resource, rc.Namespace)
	if err != nil {
		ent = store.NewInvalidEntry(err)
	}
	ok := r.store.Set(key, ent)
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		r.logger.Error(err, "")
		return
	}
	r.logger.V(4).Info("successfully created cache entry", "key", key, "entry", ent)
}

func (r *ResourceLoader) createGenericListerForResource(resource schema.GroupVersionResource, namespace string) (store.Entry, error) {
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
	return &resourceEntry{lister: lister, logger: r.logger.WithName("k8s resource entry"), watchErrHandler: watchErrHandler, cancel: cancel}, nil
}
