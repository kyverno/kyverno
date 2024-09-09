package k8sresource

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/event"
	entryevent "github.com/kyverno/kyverno/pkg/globalcontext/event"
	"github.com/kyverno/kyverno/pkg/globalcontext/store"
	controllerutils "github.com/kyverno/kyverno/pkg/utils/controller"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

type entry struct {
	lister   cache.GenericLister
	stop     func()
	gce      *kyvernov2alpha1.GlobalContextEntry
	eventGen event.Interface
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
) (store.Entry, error) {
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
	err := informer.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		if shouldUpdateStatus {
			if err := updateStatus(ctx, gce, kyvernoClient, false, "CacheSyncFailure"); err != nil {
				logger.Error(err, "failed to update status")
			}
		}

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

	group.StartWithContext(ctx, func(ctx context.Context) {
		informer.Informer().Run(ctx.Done())
	})
	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		stop()

		if shouldUpdateStatus {
			if err := updateStatus(ctx, gce, kyvernoClient, false, "CacheSyncFailure"); err != nil {
				logger.Error(err, "failed to update status")
			}
		}

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

	if shouldUpdateStatus {
		if err := updateStatus(ctx, gce, kyvernoClient, true, "CacheSyncSuccess"); err != nil {
			logger.Error(err, "failed to update status")
		}
	}

	return &entry{
		lister:   informer.Lister(),
		stop:     stop,
		eventGen: eventGen,
	}, nil
}

func (e *entry) Get() (any, error) {
	obj, err := e.lister.List(labels.Everything())
	if err != nil {
		e.eventGen.Add(entryevent.NewErrorEvent(corev1.ObjectReference{
			APIVersion: e.gce.APIVersion,
			Kind:       e.gce.Kind,
			Name:       e.gce.Name,
			Namespace:  e.gce.Namespace,
			UID:        e.gce.UID,
		}, err))
		return nil, err
	}
	return obj, nil
}

func (e *entry) Stop() {
	e.stop()
}

func updateStatus(ctx context.Context, gce *kyvernov2alpha1.GlobalContextEntry, kyvernoClient versioned.Interface, ready bool, reason string) error {
	_, err := controllerutils.UpdateStatus(ctx, gce, kyvernoClient.KyvernoV2alpha1().GlobalContextEntries(), func(latest *kyvernov2alpha1.GlobalContextEntry) error {
		if latest == nil {
			return fmt.Errorf("failed to update status: %s", gce.Name)
		}
		latest.Status.SetReady(ready, reason)
		return nil
	})
	return err
}
