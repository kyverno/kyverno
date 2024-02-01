package globalcontext

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernoinfoermers "github.com/kyverno/kyverno/pkg/client/informers/externalversions"
	kyvernov2alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2alpha1"
	kyvernov2alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/controllers"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/k8sresource"
	"github.com/kyverno/kyverno/pkg/engine/globalcontext/store"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	resyncPeriod = 15 * time.Second
	maxRetries   = 3
)

type globalContextController struct {
	logger        logr.Logger
	kyvernoClient versioned.Interface
	dynamicClient dynamic.Interface
	store         *store.Store

	queue           workqueue.RateLimitingInterface
	gctxentInformer kyvernov2alpha1informers.GlobalContextEntryInformer
	gctxentLister   kyvernov2alpha1listers.GlobalContextEntryLister
	informerSynced  cache.InformerSynced
}

func NewController(logger logr.Logger, kyvernoClient versioned.Interface, dclient dynamic.Interface, storage *store.Store) controllers.Controller {
	kyvernoinfoermer := kyvernoinfoermers.NewSharedInformerFactory(kyvernoClient, resyncPeriod)
	gctxentInformer := kyvernoinfoermer.Kyverno().V2alpha1().GlobalContextEntries()
	gctxentLister := gctxentInformer.Lister()
	informerSynced := gctxentInformer.Informer().HasSynced

	return &globalContextController{
		logger:          logger,
		kyvernoClient:   kyvernoClient,
		dynamicClient:   dclient,
		store:           storage,
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "globalcontext"),
		gctxentInformer: gctxentInformer,
		gctxentLister:   gctxentLister,
		informerSynced:  informerSynced,
	}
}

func (gc *globalContextController) Run(ctx context.Context, workers int) {
	logger := gc.logger

	defer utilruntime.HandleCrash()
	defer gc.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("GlobalContextController", ctx.Done(), gc.informerSynced) {
		return
	}

	_, _ = gc.gctxentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    gc.addGCTXEntry,
		UpdateFunc: gc.updateGCTXEntry,
		DeleteFunc: gc.deleteGCTXEntry,
	})

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, gc.worker, time.Second)
	}

	<-ctx.Done()
}

func (gc *globalContextController) addGCTXEntry(obj interface{}) {
	gc.enqueueGCTXEntry(obj.(kyvernov2alpha1.GlobalContextEntry))
}

func (gc *globalContextController) updateGCTXEntry(oldObj, newObj interface{}) {
	oldGCTXEntry := oldObj.(*kyvernov2alpha1.GlobalContextEntry)
	newGCTXEntry := newObj.(*kyvernov2alpha1.GlobalContextEntry)
	if oldGCTXEntry.ResourceVersion == newGCTXEntry.ResourceVersion {
		return
	}
	gc.enqueueGCTXEntry(newObj.(kyvernov2alpha1.GlobalContextEntry))
}

func (gc *globalContextController) deleteGCTXEntry(obj interface{}) {
	gc.logger.V(4).Info("deleting global context entry from store", "key", obj.(kyvernov2alpha1.GlobalContextEntry).Name)
	(*gc.store).Delete(obj.(kyvernov2alpha1.GlobalContextEntry).Name)
}

func (gc *globalContextController) enqueueGCTXEntry(gctxentry kyvernov2alpha1.GlobalContextEntry) {
	logger := gc.logger
	key, err := cache.MetaNamespaceKeyFunc(gctxentry)
	if err != nil {
		logger.Error(err, "failed to enqueue global context entry")
		return
	}
	gc.queue.Add(key)
}

func (gc *globalContextController) worker(ctx context.Context) {
	for gc.processNextWorkItem() {
	}
}

func (gc *globalContextController) processNextWorkItem() bool {
	key, quit := gc.queue.Get()
	if quit {
		return false
	}
	defer gc.queue.Done(key)
	err := gc.syncGCTXEntry(key.(string))
	gc.handleErr(err, key)

	return true
}

func (gc *globalContextController) syncGCTXEntry(key string) error {
	logger := gc.logger.WithName("syncGCTXEntry")
	startTime := time.Now()
	logger.V(4).Info("started syncing global context entries", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing global context entries", "key", key, "processingTime", time.Since(startTime).String())
	}()

	gctxentry, err := gc.getGCTXEntry(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	} else {
		gc.handleGCTXEntry(gctxentry)
	}
	return nil
}

func (gc *globalContextController) handleGCTXEntry(gctxentry *kyvernov2alpha1.GlobalContextEntry) {
	if gctxentry.Spec.KubernetesResource == nil && gctxentry.Spec.APICall == nil {
		gc.logger.Info("global context entry neither has K8sResource nor APICall")
		return
	}
	if gctxentry.Spec.KubernetesResource != nil && gctxentry.Spec.APICall != nil {
		gc.logger.Info("global context entry has both K8sResource and APICall")
		return
	}
	if gctxentry.Spec.KubernetesResource != nil {
		k8sresource.StoreInGlobalContext(gc.logger, gc.store, gctxentry, gc.dynamicClient)
	}
}

func (gc *globalContextController) getGCTXEntry(key string) (*kyvernov2alpha1.GlobalContextEntry, error) {
	if _, name, err := cache.SplitMetaNamespaceKey(key); err != nil {
		gc.logger.Error(err, "failed to parse global context entry name", "gctxEntryName", key)
		return nil, err
	} else {
		return gc.gctxentLister.Get(name)
	}
}

func (gc *globalContextController) handleErr(err error, key interface{}) {
	logger := gc.logger
	if err == nil {
		gc.queue.Forget(key)
		return
	}

	if gc.queue.NumRequeues(key) < maxRetries {
		logger.Error(err, "failed to sync global context entry", "key", key)
		gc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	logger.V(2).Info("dropping global context entry out of queue", "key", key)
	gc.queue.Forget(key)
}
