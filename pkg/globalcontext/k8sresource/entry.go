package k8sresource

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type entry struct {
	lister      cache.GenericLister
	stop        func()
	gce         *kyvernov2alpha1.GlobalContextEntry
	eventGen    event.Interface
	projections []store.Projection
	jp          jmespath.Interface

	objectsMu sync.RWMutex
	objects   map[string]interface{}
	projected map[string]interface{}
}

func New(
	ctx context.Context,
	gce *kyvernov2alpha1.GlobalContextEntry,
	eventGen event.Interface,
	kubeClient kubernetes.Interface,
	dClient dynamic.Interface,
	logger logr.Logger,
	gvr schema.GroupVersionResource,
	namespace string,
	jp jmespath.Interface,
) (store.Entry, error) {
	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	factory := informers.NewSharedInformerFactoryWithOptions(kubeClient, 0, informers.WithNamespace(namespace))
	informer, err := factory.ForResource(gvr)
	if err != nil {
		logger.Info("no built-in informer found, use dynamic informer", "gvr", gvr)
		informer = dynamicinformer.NewFilteredDynamicInformer(dClient, gvr, namespace, 0, nil, nil)
	}

	var group wait.Group
	ctx, cancel := context.WithCancel(ctx)
	stop := func() {
		cancel()
		// Wait for the group to terminate
		group.Wait()
	}

	err = informer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		eventErr := fmt.Errorf("failed to run informer for %s", gvr)
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
		objects:     make(map[string]interface{}),
		projected:   make(map[string]interface{}),
	}

	_, err = informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    e.handleAdd,
		UpdateFunc: func(oldObj, newObj interface{}) { e.handleUpdate(newObj) },
		DeleteFunc: e.handleDelete,
	})
	if err != nil {
		return nil, err
	}

	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})

	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stop()
		err := fmt.Errorf("failed to sync cache for %s", gvr)
		eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: gce.APIVersion,
			Kind:       gce.Kind,
			Name:       gce.Name,
			Namespace:  gce.Namespace,
			UID:        gce.UID,
		}, err))
		return nil, err
	}

	return e, nil
}

func (e *entry) handleAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to get key for object: %w", err)))
		return
	}

	jsonData, err := json.Marshal(obj)
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to marshal object: %w", err)))
		return
	}

	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to unmarshal object: %w", err)))
		return
	}

	e.objectsMu.Lock()
	e.objects[key] = data
	e.objectsMu.Unlock()

	e.recomputeProjections()
}

func (e *entry) handleUpdate(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to get key for updated object: %w", err)))
		return
	}

	jsonData, err := json.Marshal(obj)
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to marshal object: %w", err)))
		return
	}

	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to unmarshal object: %w", err)))
		return
	}

	e.objectsMu.Lock()
	e.objects[key] = data
	e.objectsMu.Unlock()

	e.recomputeProjections()
}

func (e *entry) handleDelete(obj interface{}) {
	deletedObj, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = deletedObj.Obj
	}

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, fmt.Errorf("failed to get key for deleted object: %w", err)))
		return
	}

	e.objectsMu.Lock()
	delete(e.objects, key)
	e.objectsMu.Unlock()

	e.recomputeProjections()
}

func (e *entry) recomputeProjections() {
	e.objectsMu.RLock()
	list := make([]interface{}, 0, len(e.objects))
	for _, obj := range e.objects {
		list = append(list, obj)
	}
	e.objectsMu.RUnlock()

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
		e.objectsMu.Lock()
		e.projected[proj.Name] = result
		e.objectsMu.Unlock()
	}
}

func (e *entry) Get(projection string) (any, error) {
	e.objectsMu.RLock()
	defer e.objectsMu.RUnlock()

	if projection == "" {
		list := make([]interface{}, 0, len(e.objects))
		for _, obj := range e.objects {
			list = append(list, obj)
		}
		return list, nil
	}

	if result, ok := e.projected[projection]; ok {
		return result, nil
	}
	return nil, fmt.Errorf("projection %q not found", projection)
}

func (e *entry) Stop() {
	e.stop()
}
