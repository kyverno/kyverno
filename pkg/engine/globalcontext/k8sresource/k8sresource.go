package k8sresource

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 15 * time.Second
)

type entry struct {
	lister cache.GenericLister
}

// TODO: error handling
func New(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource) (*entry, error) {
	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
	// TODO: account for namespace here
	informer := dynamicinformer.NewFilteredDynamicInformer(client, gvr, metav1.NamespaceAll, 10*time.Minute, indexers, nil)
	var group wait.Group
	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		// TODO
		// stopInformer()
		return nil, fmt.Errorf("failed to wait for cache sync: %s", gvr.Resource)
	}
	return &entry{
		lister: informer.Lister(),
	}, nil
}

func (e *entry) Get() (interface{}, error) {
	// TODO
	return nil, nil
}

func (e *entry) Stop() {
	// TODO
}

// func (re *resourceEntry) Get() (interface{}, error) {
// 	re.Lock()
// 	defer re.Unlock()

// 	re.logger.V(4).Info("fetching data from resource cache entry")
// 	if re.watchErrHandler.Error() != nil {
// 		re.logger.Error(re.watchErrHandler.Error(), "failed to fetch data from entry")
// 		return nil, re.watchErrHandler.Error()
// 	}
// 	obj, err := re.lister.List(labels.Everything())
// 	if err != nil {
// 		re.logger.Error(err, "failed to fetch data from entry")
// 		return nil, err
// 	}
// 	re.logger.V(6).Info("cache entry data", "total fetched:", len(obj))
// 	for _, o := range obj {
// 		metadata, err := meta.Accessor(o)
// 		if err != nil {
// 			continue
// 		}
// 		re.logger.V(6).Info("cache entry data", "name", metadata.GetName(), "namespace", metadata.GetNamespace())
// 	}
// 	return obj, nil
// }

// func (re *resourceEntry) Stop() {
// 	re.cancel()
// }

// func StoreInGlobalContext(logger logr.Logger, gctxStore *store.Store, entry *v2alpha1.GlobalContextEntry, client dynamic.Interface) {
// 	if entry.Spec.KubernetesResource == nil {
// 		return
// 	}
// 	rc := entry.Spec.KubernetesResource
// 	resource := schema.GroupVersionResource{
// 		Group:    rc.Group,
// 		Version:  rc.Version,
// 		Resource: rc.Resource,
// 	}
// 	key := entry.Name
// 	ent, err := createGenericListerForResource(logger, rc.Namespace, resource, client)
// 	if err != nil {
// 		ent = invalid.New(err)
// 	}
// 	ok := (*gctxStore).Set(key, ent)
// 	if !ok {
// 		err := fmt.Errorf("failed to create cache entry key=%s", key)
// 		logger.Error(err, "")
// 		return
// 	}
// 	logger.V(4).Info("successfully created cache entry", "key", key, "entry", ent)
// }

// func createGenericListerForResource(logger logr.Logger, namespace string, resource schema.GroupVersionResource, client dynamic.Interface) (store.Entry, error) {
// 	informer := dynamicinformer.NewFilteredDynamicInformer(client, resource, namespace, resyncPeriod, k8scache.Indexers{k8scache.NamespaceIndex: k8scache.MetaNamespaceIndexFunc}, nil)
// 	watchErrHandler := NewWatchErrorHandler(logger, resource, namespace)
// 	err := informer.Informer().SetWatchErrorHandler(watchErrHandler.WatchErrorHandlerFunction())
// 	if err != nil {
// 		logger.Error(err, "failed to add watch error handler")
// 		return nil, err
// 	}

// 	ctx, cancel := context.WithCancel(context.TODO())
// 	go informer.Informer().Run(ctx.Done())
// 	if !k8scache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
// 		cancel()
// 		err := errors.New("resource informer cache failed to sync")
// 		logger.Error(err, "")
// 		return nil, err
// 	}

// 	var lister k8scache.GenericNamespaceLister
// 	if len(namespace) == 0 {
// 		lister = informer.Lister()
// 	} else {
// 		lister = informer.Lister().ByNamespace(namespace)
// 	}
// 	return &resourceEntry{lister: lister, logger: logger.WithName("k8s resource entry"), watchErrHandler: watchErrHandler, cancel: cancel}, nil
// }
