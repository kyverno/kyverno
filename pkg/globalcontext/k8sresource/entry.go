package k8sresource

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// cacheSyncTimeout bounds the initial cache sync in New. Failure modes that
// do not trigger the watch error handler promptly (e.g. an unreachable API
// server retried internally by client-go) must not block the globalcontext
// controller's single reconcile worker indefinitely; the workqueue retries
// with backoff on error. Callers can enforce a shorter bound through the
// deadline of the context passed to New.
const (
	cacheSyncTimeout        = 30 * time.Second
	watchErrorEventInterval = time.Minute
)

// shouldReportWatchError bounds post-sync warning events to one per interval.
// Initial sync failures are always reported separately by the watch handler.
func shouldReportWatchError(now, last time.Time) bool {
	return last.IsZero() || !now.Before(last.Add(watchErrorEventInterval))
}

type entry struct {
	lister      cache.GenericLister
	stop        func()
	gce         *kyvernov2beta1.GlobalContextEntry
	eventGen    event.Interface
	projections []store.Projection
	jp          jmespath.Interface

	// projected stores pre-computed projection results
	// Only projections are cached since JMESPath computation is expensive
	// Raw data is read directly from the lister to avoid memory duplication
	projectedMu sync.RWMutex
	projected   map[string]interface{}
}

// newOnceStop returns the entry's stop function: it cancels the informer
// context and waits for the background group to terminate, exactly once, so
// repeated or concurrent Stop calls are safe and never re-run the teardown.
func newOnceStop(cancel context.CancelFunc, group *wait.Group) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			// Wait for the group to terminate
			group.Wait()
		})
	}
}

func New(
	ctx context.Context,
	gce *kyvernov2beta1.GlobalContextEntry,
	eventGen event.Interface,
	dClient dynamic.Interface,
	logger logr.Logger,
	gvr schema.GroupVersionResource,
	namespace string,
	jp jmespath.Interface,
) (store.Entry, error) {
	return newWithWatchErrorObserver(ctx, gce, eventGen, dClient, logger, gvr, namespace, jp, nil)
}

// newWithWatchErrorObserver lets regression tests observe callbacks that are
// intentionally suppressed by event throttling. Production passes nil.
func newWithWatchErrorObserver(
	ctx context.Context,
	gce *kyvernov2beta1.GlobalContextEntry,
	eventGen event.Interface,
	dClient dynamic.Interface,
	logger logr.Logger,
	gvr schema.GroupVersionResource,
	namespace string,
	jp jmespath.Interface,
	observeWatchError func(error),
) (store.Entry, error) {
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	// Use DynamicInformer for all resources
	// DynamicInformer returns *unstructured.Unstructured which can be used directly for JMESPath queries
	informer := dynamicinformer.NewFilteredDynamicInformer(dClient, gvr, namespace, 0, nil, nil)
	logger.V(4).Info("using DynamicInformer", "gvr", gvr)

	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)
	stop := newOnceStop(cancel, &group)

	// Coordinate the transition from initial cache synchronization to normal
	// operation with watch errors. A failing initial LIST must still abort New
	// promptly, but errors after a successful sync must not kill the informer.
	// The mutex also prevents returning an already-cancelled entry if an error
	// races with the completion of initial synchronization.
	var startupMu sync.Mutex
	startupComplete := false
	var lastWatchErrorEvent time.Time

	err := informer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		// Observe even suppressed errors so tests can verify both callbacks.
		if observeWatchError != nil {
			observeWatchError(err)
		}

		// Fail fast only before initial cache synchronization. Once synced,
		// leave retries and relisting to the client-go reflector (#18180).
		// Serialize the decision with New's startup completion and throttle
		// recurring diagnostics so an API outage cannot flood Warning Events.
		// Never call stop() here: it would join the reflector's own goroutine.
		startupMu.Lock()
		initialFailure := !startupComplete && !informer.Informer().HasSynced()
		if initialFailure {
			cancel()
		}
		now := time.Now()
		reportError := initialFailure || shouldReportWatchError(now, lastWatchErrorEvent)
		if reportError {
			lastWatchErrorEvent = now
		}
		startupMu.Unlock()

		if reportError {
			eventErr := fmt.Errorf("failed to run informer for %s: %w", gvr, err)
			eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
				APIVersion: gce.APIVersion,
				Kind:       gce.Kind,
				Name:       gce.Name,
				Namespace:  gce.Namespace,
				UID:        gce.UID,
			}, eventErr))
		}
	})
	if err != nil {
		logger.Error(err, "failed to set watch error handler")
		return nil, err
	}

	var projections []store.Projection
	if len(gce.Spec.Projections) > 0 {
		for _, p := range gce.Spec.Projections {
			jpQuery, err := jp.Query(p.JMESPath)
			if err != nil {
				return nil, fmt.Errorf("failed to parse jmespath query for projection %q: %w", p.Name, err)
			}
			projections = append(projections, store.Projection{
				Name: p.Name,
				JP:   jpQuery,
			})
		}
	}

	e := &entry{
		lister:      informer.Lister(),
		stop:        stop,
		gce:         gce,
		eventGen:    eventGen,
		projections: projections,
		jp:          jp,
		projected:   make(map[string]interface{}),
	}

	// Only add event handlers if projections are defined
	// This avoids unnecessary processing when projections are not used
	if len(projections) > 0 {
		_, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { e.recomputeProjections() },
			UpdateFunc: func(oldObj, newObj interface{}) { e.recomputeProjections() },
			DeleteFunc: func(obj interface{}) { e.recomputeProjections() },
		})
		if err != nil {
			return nil, err
		}
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})

	syncCtx, syncCancel := context.WithTimeout(ctx, cacheSyncTimeout)
	defer syncCancel()
	if !cache.WaitForCacheSync(syncCtx.Done(), informer.Informer().HasSynced) {
		stop()
		var err error
		if cause := ctx.Err(); cause != nil {
			// The informer context ended: either the caller cancelled it, or the
			// watch error handler observed a list/watch failure (e.g. the
			// referenced resource does not exist in the cluster).
			err = fmt.Errorf("failed to sync cache for %s: informer stopped (%w), the resource may not exist in the cluster", gvr, cause)
		} else {
			err = fmt.Errorf("failed to sync cache for %s: timed out after %s waiting for the informer to sync", gvr, cacheSyncTimeout)
		}
		eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: gce.APIVersion,
			Kind:       gce.Kind,
			Name:       gce.Name,
			Namespace:  gce.Namespace,
			UID:        gce.UID,
		}, err))
		return nil, err
	}

	// Publish a successfully synchronized entry only after closing the
	// watch-error/startup race. If a pre-sync failure cancelled the context
	// just as WaitForCacheSync reported success, do not retain a dead entry.
	startupMu.Lock()
	startupComplete = true
	startupErr := ctx.Err()
	startupMu.Unlock()
	if startupErr != nil {
		stop()
		return nil, fmt.Errorf("failed to sync cache for %s: informer stopped (%w)", gvr, startupErr)
	}

	// Compute initial projections after cache sync
	if len(projections) > 0 {
		e.recomputeProjections()
	}

	// A cancellation may occur while computing initial projections. Never
	// publish an entry whose backing informer has already stopped.
	if err := ctx.Err(); err != nil {
		stop()
		return nil, fmt.Errorf("failed to initialize cache for %s: informer stopped (%w)", gvr, err)
	}

	return e, nil
}

// listObjects retrieves all objects from the lister and returns them as a slice of map[string]interface{}
// Since we use DynamicInformer, objects are *unstructured.Unstructured and can be used directly
func (e *entry) listObjects() ([]interface{}, error) {
	objs, err := e.lister.List(labels.Everything())
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	list := make([]interface{}, 0, len(objs))
	for _, obj := range objs {
		// DynamicInformer returns *unstructured.Unstructured
		// We can use its Object field directly which is already map[string]interface{}
		if u, ok := obj.(*unstructured.Unstructured); ok {
			list = append(list, u.Object)
		}
	}
	return list, nil
}

func (e *entry) recomputeProjections() {
	list, err := e.listObjects()
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, err))
		return
	}

	for _, proj := range e.projections {
		result, err := proj.JP.Search(list)
		if err != nil {
			e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
				APIVersion: e.gce.APIVersion,
				Kind:       e.gce.Kind,
				Name:       e.gce.Name,
				Namespace:  e.gce.Namespace,
				UID:        e.gce.UID,
			}, fmt.Errorf("failed to apply projection %q: %w", proj.Name, err)))
			continue
		}
		e.projectedMu.Lock()
		e.projected[proj.Name] = result
		e.projectedMu.Unlock()
	}
}

func (e *entry) Get(projection string) (any, error) {
	// If no projection specified, return all objects directly from lister
	if projection == "" {
		return e.listObjects()
	}

	// Return pre-computed projection result
	e.projectedMu.RLock()
	defer e.projectedMu.RUnlock()

	if result, ok := e.projected[projection]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("projection %q not found", projection)
}

func (e *entry) Stop() {
	e.stop()
}
