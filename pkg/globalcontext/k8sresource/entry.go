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
	cacheSyncTimeout = 30 * time.Second
	// projectionCoalesceInterval bounds the time spent collecting a burst of
	// informer events before refreshing from the latest cache snapshot. Unlike
	// a quiet-period debounce, sustained traffic cannot defer a refresh forever.
	projectionCoalesceInterval = 25 * time.Millisecond
)

// notifyProjectionChange never blocks the informer event handler. A buffered
// notification records that at least one refresh is needed, even if events
// arrive while a previous projection evaluation is still running.
func notifyProjectionChange(changes chan<- struct{}) {
	select {
	case changes <- struct{}{}:
	default:
	}
}

// runProjectionWorker serializes expensive full-cache evaluations and batches
// notifications over a fixed, bounded window. Notifications during an active
// evaluation remain pending for the next iteration.
func runProjectionWorker(ctx context.Context, changes <-chan struct{}, refresh func()) {
	// Reuse one timer across bursts to avoid allocations and GC pressure on
	// frequently updated resources. Stop it before the first notification.
	timer := time.NewTimer(projectionCoalesceInterval)
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-changes:
		}

		timer.Reset(projectionCoalesceInterval)
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if ctx.Err() != nil {
			return
		}
		// Events already queued during the coalescing window are covered by
		// the upcoming full-cache snapshot. Preserve only notifications
		// arriving during (or after) the evaluation for the next iteration.
		select {
		case <-changes:
		default:
		}
		refresh()
	}
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
	var changes chan struct{}
	if len(gce.Spec.Projections) > 0 {
		changes = make(chan struct{}, 1)
	}
	return newWithChanges(ctx, gce, eventGen, dClient, logger, gvr, namespace, jp, changes)
}

// newWithChanges accepts the notification channel so integration tests can
// observe informer callback delivery during the initial projection snapshot.
func newWithChanges(
	ctx context.Context,
	gce *kyvernov2beta1.GlobalContextEntry,
	eventGen event.Interface,
	dClient dynamic.Interface,
	logger logr.Logger,
	gvr schema.GroupVersionResource,
	namespace string,
	jp jmespath.Interface,
	changes chan struct{},
) (store.Entry, error) {
	// Keep this internal helper safe for callers that do not supply a channel.
	// Without it, projected entries would silently miss informer changes.
	if len(gce.Spec.Projections) > 0 && changes == nil {
		changes = make(chan struct{}, 1)
	}

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

	err := informer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		eventErr := fmt.Errorf("failed to run informer for %s", gvr)
		eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: gce.APIVersion,
			Kind:       gce.Kind,
			Name:       gce.Name,
			Namespace:  gce.Namespace,
			UID:        gce.UID,
		}, eventErr))

		// Only cancel the context here, never call stop(). This handler runs
		// on the reflector's goroutine, which informer.Informer().Run (and
		// therefore stop's group.Wait) transitively waits on: calling stop()
		// from here self-deadlocks, wedging the entry forever and permanently
		// blocking the globalcontext controller's single reconcile worker, so
		// one entry referencing a non-existent CRD prevents every other
		// GlobalContextEntry from loading (#15856). Cancelling the context
		// lets the informer wind down; WaitForCacheSync below observes the
		// cancellation and the error path performs the full stop().
		cancel()
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

	// A single buffered notification replaces synchronous full-cache scans in
	// informer callbacks. No projection work is started during initial LIST.
	var registration cache.ResourceEventHandlerRegistration
	if len(projections) > 0 {
		registration, err = informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { notifyProjectionChange(changes) },
			UpdateFunc: func(oldObj, newObj interface{}) { notifyProjectionChange(changes) },
			DeleteFunc: func(obj interface{}) { notifyProjectionChange(changes) },
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
	// The informer store can report synced before its registered handlers
	// process the initial LIST. Wait for both to prevent a delayed initial
	// Add callback from masquerading as a post-sync change notification.
	synced := []cache.InformerSynced{informer.Informer().HasSynced}
	if registration != nil {
		synced = append(synced, registration.HasSynced)
	}
	if !cache.WaitForCacheSync(syncCtx.Done(), synced...) {
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

	// The initial LIST has already populated the cache. Discard its dirty
	// notification before taking the first complete snapshot; events occurring
	// during the snapshot are buffered and handled by the worker below.
	if len(projections) > 0 {
		select {
		case <-changes:
		default:
		}
		e.recomputeProjections()
		group.StartWithContext(ctx, func(ctx context.Context) {
			runProjectionWorker(ctx, changes, e.recomputeProjections)
		})
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

	updated := make(map[string]interface{}, len(e.projections))
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
		updated[proj.Name] = result
	}

	// Publish all successful results under one lock. Preserve the previous
	// value of a failed projection, matching the existing failure semantics,
	// instead of exposing partial updates between successful evaluations.
	e.projectedMu.Lock()
	next := make(map[string]interface{}, len(e.projected)+len(updated))
	for name, value := range e.projected {
		next[name] = value
	}
	for name, value := range updated {
		next[name] = value
	}
	e.projected = next
	e.projectedMu.Unlock()
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
