package k8sresource

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

type entry struct {
	sync.RWMutex
	dataMap     map[string]any
	stop        func()
	gce         *kyvernov2alpha1.GlobalContextEntry
	eventGen    event.Interface
	projections []store.Projection
	jp          jmespath.Interface
}

// TODO: Handle Kyverno Pod Ready State
func New(
	ctx context.Context,
	gce *kyvernov2alpha1.GlobalContextEntry,
	eventGen event.Interface,
	client dynamic.Interface,
	kyvernoClient versioned.Interface,
	logger logr.Logger,
	gvr schema.GroupVersionResource,
	namespace string,
	shouldUpdateStatus bool,
	jp jmespath.Interface,
) (store.Entry, error) {
	e := &entry{
		dataMap:  make(map[string]any),
		gce:      gce,
		eventGen: eventGen,
		jp:       jp,
	}

	projections := make([]store.Projection, 0)
	for _, p := range gce.Spec.Projections {
		jpQuery, err := jp.Query(p.JMESPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse jmespath query for projection '%s': %w", p.Name, err)
		}
		projections = append(projections, store.Projection{
			Name: p.Name,
			JP:   jpQuery,
		})
	}
	e.projections = projections

	indexers := cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}
	informer := dynamicinformer.NewFilteredDynamicInformer(client, gvr, namespace, 0, indexers, nil)

	_, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.addResource,
		UpdateFunc: e.updateResource,
		DeleteFunc: e.deleteResource,
	})
	if err != nil {
		return nil, err
	}

	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		// Send stop signal to informer's goroutine
		cancel()
		// Wait for the group to terminate
		group.Wait()
	}
	e.stop = stop

	err = informer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if shouldUpdateStatus {
			if updateErr := updateStatus(ctx, gce, kyvernoClient, false, "CacheSyncFailure"); updateErr != nil {
				logger.Error(updateErr, "failed to update status")
			}
		}

		eventErr := fmt.Errorf("failed to run informer for %s: %w", gvr, err)
		eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: gce.APIVersion,
			Kind:       gce.Kind,
			Name:       gce.Name,
			Namespace:  gce.Namespace,
			UID:        gce.UID,
		}, eventErr))

		stop()
	})
	if err != nil {
		logger.Error(err, "failed to set watch error handler")
		return nil, err
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})

	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stop()

		if shouldUpdateStatus {
			if updateErr := updateStatus(ctx, gce, kyvernoClient, false, "CacheSyncFailure"); updateErr != nil {
				logger.Error(updateErr, "failed to update status")
			}
		}

		syncErr := fmt.Errorf("failed to sync cache for %s", gvr)
		eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: gce.APIVersion,
			Kind:       gce.Kind,
			Name:       gce.Name,
			Namespace:  gce.Namespace,
			UID:        gce.UID,
		}, syncErr))

		return nil, syncErr
	}

	if shouldUpdateStatus {
		if updateErr := updateStatus(ctx, gce, kyvernoClient, true, "CacheSyncSuccess"); updateErr != nil {
			logger.Error(updateErr, "failed to update status")
		}
	}

	return e, nil
}

func (e *entry) addResource(obj interface{}) {
	e.Lock()
	defer e.Unlock()
	e.processResource(obj)
}

func (e *entry) updateResource(oldObj, newObj interface{}) {
	e.Lock()
	defer e.Unlock()
	e.processResource(newObj)
}

func (e *entry) deleteResource(obj interface{}) {
	e.Lock()
	defer e.Unlock()
	deletedObj, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = deletedObj.Obj
	}
	e.processResource(obj)
}

func (e *entry) processResource(obj interface{}) {
	e.updateData(obj.(runtime.Object))
}

func (e *entry) updateData(object runtime.Object) {
	e.Lock()
	defer e.Unlock()

	e.dataMap[""] = object

	for _, projection := range e.projections {
		res, err := projection.JP.Search(object)
		if err != nil {
			e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
				APIVersion: e.gce.APIVersion,
				Kind:       e.gce.Kind,
				Name:       e.gce.Name,
				Namespace:  e.gce.Namespace,
				UID:        e.gce.UID,
			}, fmt.Errorf("failed to apply projection '%s': %w", projection.Name, err)))
			continue
		}
		e.dataMap[projection.Name] = res
	}
}

func (e *entry) Get(projection string) (any, error) {
	e.RLock()
	defer e.RUnlock()

	data, ok := e.dataMap[projection]
	if !ok {
		return nil, fmt.Errorf("projection '%s' not found", projection)
	}
	return data, nil
}

func (e *entry) Stop() {
	e.stop()
}

func updateStatus(ctx context.Context, gce *kyvernov2alpha1.GlobalContextEntry, kyvernoClient versioned.Interface, ready bool, reason string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latestGCE, getErr := kyvernoClient.KyvernoV2alpha1().GlobalContextEntries().Get(ctx, gce.GetName(), metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		updateErr := controllerutils.UpdateStatus(ctx, latestGCE, kyvernoClient.KyvernoV2alpha1().GlobalContextEntries(), func(latest *kyvernov2alpha1.GlobalContextEntry) error {
			if latest == nil {
				return fmt.Errorf("failed to update status: %s", gce.GetName())
			}
			latest.Status.SetReady(ready, reason)
			return nil
		}, nil)
		return updateErr
	})
	return retryErr
}
