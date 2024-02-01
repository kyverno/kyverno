package k8sresource

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/invalid"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	k8scache "k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 15 * time.Second
)

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

func StoreInGlobalContext(logger logr.Logger, gctxStore *store.Store, entry *v2alpha1.GlobalContextEntry, client dynamic.Interface) {
	if entry.Spec.KubernetesResource == nil {
		return
	}
	rc := entry.Spec.KubernetesResource
	resource := schema.GroupVersionResource{
		Group:    rc.Group,
		Version:  rc.Version,
		Resource: rc.Resource,
	}
	key := entry.Name
	ent, err := createGenericListerForResource(logger, rc.Namespace, resource, client)
	if err != nil {
		ent = invalid.New(err)
	}
	ok := (*gctxStore).Set(key, ent)
	if !ok {
		err := fmt.Errorf("failed to create cache entry key=%s", key)
		logger.Error(err, "")
		return
	}
	logger.V(4).Info("successfully created cache entry", "key", key, "entry", ent)
}

func createGenericListerForResource(logger logr.Logger, namespace string, resource schema.GroupVersionResource, client dynamic.Interface) (store.Entry, error) {
	informer := dynamicinformer.NewFilteredDynamicInformer(client, resource, namespace, resyncPeriod, k8scache.Indexers{k8scache.NamespaceIndex: k8scache.MetaNamespaceIndexFunc}, nil)
	watchErrHandler := NewWatchErrorHandler(logger, resource, namespace)
	err := informer.Informer().SetWatchErrorHandler(watchErrHandler.WatchErrorHandlerFunction())
	if err != nil {
		logger.Error(err, "failed to add watch error handler")
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.TODO())
	go informer.Informer().Run(ctx.Done())
	if !k8scache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		cancel()
		err := errors.New("resource informer cache failed to sync")
		logger.Error(err, "")
		return nil, err
	}

	var lister k8scache.GenericNamespaceLister
	if len(namespace) == 0 {
		lister = informer.Lister()
	} else {
		lister = informer.Lister().ByNamespace(namespace)
	}
	return &resourceEntry{lister: lister, logger: logger.WithName("k8s resource entry"), watchErrHandler: watchErrHandler, cancel: cancel}, nil
}
