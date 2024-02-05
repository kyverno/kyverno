package k8sresource

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type entry struct {
	lister cache.GenericLister
	stop   func()
}

// TODO: error handling
func New(ctx context.Context, client dynamic.Interface, gvr schema.GroupVersionResource, namespace string) (*entry, error) {
	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	informer := dynamicinformer.NewFilteredDynamicInformer(client, gvr, namespace, 0, indexers, nil)
	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		// Send stop signal to informer's goroutine
		cancel()
		// Wait for the group to terminate
		group.Wait()
	}
	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stop()
		return nil, fmt.Errorf("failed to wait for cache sync: %s", gvr.Resource)
	}
	return &entry{
		lister: informer.Lister(),
		stop:   stop,
	}, nil
}

func (e *entry) Get() (any, error) {
	obj, err := e.lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (e *entry) Stop() {
	e.stop()
}
