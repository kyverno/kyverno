package policy

import (
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/background/gpol"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	policiesv1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1beta1"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	"github.com/kyverno/kyverno/pkg/utils/generator"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"go.uber.org/multierr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/util/workqueue"
)

const (
	// maxRetries is the number of times a Policy will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a deployment is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

// policyController is responsible for synchronizing Policy objects stored
// in the system with the corresponding policy violations
type policyController struct {
	client        dclient.Interface
	kyvernoClient versioned.Interface

	gpolInformer  policiesv1beta1informers.GeneratingPolicyInformer
	ngpolInformer policiesv1beta1informers.NamespacedGeneratingPolicyInformer
	mpolInformer  policiesv1beta1informers.MutatingPolicyInformer
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer

	eventGen      event.Interface
	eventRecorder events.EventRecorder

	// Policies that need to be synced
	queue workqueue.TypedRateLimitingInterface[any]

	// gpolLister can list/get generating policy from the shared informer's store
	gpolLister policiesv1beta1listers.GeneratingPolicyLister

	// ngpolLister can list/get namespaced generating policy from the shared informer's store
	ngpolLister policiesv1beta1listers.NamespacedGeneratingPolicyLister

	// mpolLister can list/get mutating policy from the shared informer's store
	mpolLister policiesv1beta1listers.MutatingPolicyLister

	// nmpolLister can list/get namespaced mutating policy from the shared informer's store
	nmpolLister policiesv1beta1listers.NamespacedMutatingPolicyLister

	informersSynced []cache.InformerSynced

	reconcilePeriod time.Duration

	log logr.Logger

	metricsConfig metrics.MetricsConfigManager

	urGenerator generator.UpdateRequestGenerator

	watchManager *gpol.WatchManager

	gpolEngine   gpolengine.Engine
	gpolProvider gpolengine.Provider

	// mapper
	restMapper meta.RESTMapper
}

// NewPolicyController create a new PolicyController
func NewPolicyController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	gpolInformer policiesv1beta1informers.GeneratingPolicyInformer,
	ngpolInformer policiesv1beta1informers.NamespacedGeneratingPolicyInformer,
	mpolInformer policiesv1beta1informers.MutatingPolicyInformer,
	nmpolInformer policiesv1beta1informers.NamespacedMutatingPolicyInformer,
	urInformer kyvernov2informers.UpdateRequestInformer,
	eventGen event.Interface,
	log logr.Logger,
	reconcilePeriod time.Duration,
	metricsConfig metrics.MetricsConfigManager,
	urGenerator generator.UpdateRequestGenerator,
	watchManager *gpol.WatchManager,
	gpolEngine gpolengine.Engine,
	gpolProvider gpolengine.Provider,
) (*policyController, error) {
	// Event broad caster
	eventInterface := client.GetEventsInterface()
	eventBroadcaster := events.NewBroadcaster(
		&events.EventSinkImpl{
			Interface: eventInterface,
		},
	)
	eventBroadcaster.StartStructuredLogging(0)
	stopCh := make(chan struct{})
	eventBroadcaster.StartRecordingToSink(stopCh)

	pc := policyController{
		client:        client,
		kyvernoClient: kyvernoClient,
		gpolInformer:  gpolInformer,
		ngpolInformer: ngpolInformer,
		mpolInformer:  mpolInformer,
		nmpolInformer: nmpolInformer,
		eventGen:      eventGen,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, "policy_controller"),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "policy"},
		),
		reconcilePeriod: reconcilePeriod,
		metricsConfig:   metricsConfig,
		log:             log,
		urGenerator:     urGenerator,
		watchManager:    watchManager,
		gpolEngine:      gpolEngine,
		gpolProvider:    gpolProvider,
	}
	apiGroupResources, _ := restmapper.GetAPIGroupResources(client.GetKubeClient().Discovery())
	pc.restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)

	pc.gpolLister = gpolInformer.Lister()
	pc.ngpolLister = ngpolInformer.Lister()
	pc.mpolLister = mpolInformer.Lister()
	pc.nmpolLister = nmpolInformer.Lister()

	pc.informersSynced = []cache.InformerSynced{gpolInformer.Informer().HasSynced, ngpolInformer.Informer().HasSynced, mpolInformer.Informer().HasSynced, nmpolInformer.Informer().HasSynced, urInformer.Informer().HasSynced}

	return &pc, nil
}

func (pc *policyController) addPolicy(obj interface{}) {
	logger := pc.log
	policy := castPolicy(obj)
	if policy == nil {
		return
	}
	logger.V(2).Info("policy created", "uid", policy.GetUID(), "kind", policy.GetKind(), "namespace", policy.GetNamespace(), "name", policy.GetName())

	logger.V(4).Info("queuing policy for background processing", "name", policy.GetName())
	pc.enqueuePolicy(policy)
}

func (pc *policyController) updatePolicy(old, new interface{}) {
	logger := pc.log
	oldPolicy := castPolicy(old)
	newPolicy := castPolicy(new)
	if oldPolicy == nil || newPolicy == nil {
		return
	}

	oldgpol := oldPolicy.AsGeneratingPolicy()
	newgpol := newPolicy.AsGeneratingPolicy()
	if oldgpol != nil && newgpol != nil {
		if datautils.DeepEqual(oldgpol.Spec, newgpol.Spec) {
			return
		}
		// If the policy is updated to disable synchronization, we need to remove the watchers.
		if oldgpol.Spec.SynchronizationEnabled() && !newgpol.Spec.SynchronizationEnabled() {
			logger.V(2).Info("removing watchers for generating policy", "name", oldgpol.GetName())
			pc.watchManager.RemoveWatchersForPolicy(oldgpol.GetName(), false)
		}
	}

	oldngpol := oldPolicy.AsNamespacedGeneratingPolicy()
	newngpol := newPolicy.AsNamespacedGeneratingPolicy()
	if oldngpol != nil && newngpol != nil {
		if datautils.DeepEqual(oldngpol.Spec, newngpol.Spec) {
			return
		}
		// If the policy is updated to disable synchronization, we need to remove the watchers.
		policyKey := oldngpol.GetNamespace() + "/" + oldngpol.GetName()
		if oldngpol.Spec.SynchronizationEnabled() && !newngpol.Spec.SynchronizationEnabled() {
			logger.V(2).Info("removing watchers for namespaced generating policy", "name", policyKey)
			pc.watchManager.RemoveWatchersForPolicy(policyKey, false)
		}
	}

	oldmpol := oldPolicy.AsMutatingPolicy()
	newmpol := newPolicy.AsMutatingPolicy()
	if oldmpol != nil && newmpol != nil {
		if datautils.DeepEqual(oldmpol.Spec, newmpol.Spec) {
			return
		}
	}

	oldnmpol := oldPolicy.AsNamespacedMutatingPolicy()
	newnmpol := newPolicy.AsNamespacedMutatingPolicy()
	if oldnmpol != nil && newnmpol != nil {
		if datautils.DeepEqual(oldnmpol.Spec, newnmpol.Spec) {
			return
		}
	}

	logger.V(2).Info("updating policy", "name", oldPolicy.GetName())
	pc.enqueuePolicy(newPolicy)
}

func (pc *policyController) deletePolicy(obj interface{}) {
	logger := pc.log
	var p engineapi.GenericPolicy

	switch kubeutils.GetObjectWithTombstone(obj).(type) {
	case *policiesv1beta1.GeneratingPolicy:
		gpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.GeneratingPolicy)
		if gpol.Spec.OrphanDownstreamOnPolicyDeleteEnabled() {
			pc.watchManager.RemoveWatchersForPolicy(gpol.GetName(), false)
		} else {
			pc.watchManager.RemoveWatchersForPolicy(gpol.GetName(), true)
		}
		p = engineapi.NewGeneratingPolicy(gpol)
	case *policiesv1beta1.NamespacedGeneratingPolicy:
		ngpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.NamespacedGeneratingPolicy)
		policyKey := ngpol.GetNamespace() + "/" + ngpol.GetName()
		if ngpol.Spec.OrphanDownstreamOnPolicyDeleteEnabled() {
			pc.watchManager.RemoveWatchersForPolicy(policyKey, false)
		} else {
			pc.watchManager.RemoveWatchersForPolicy(policyKey, true)
		}
		p = engineapi.NewNamespacedGeneratingPolicy(ngpol)
	case *policiesv1beta1.MutatingPolicy:
		mpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.MutatingPolicy)
		p = engineapi.NewMutatingPolicy(mpol)
	case *policiesv1beta1.NamespacedMutatingPolicy:
		nmpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1beta1.NamespacedMutatingPolicy)
		p = engineapi.NewNamespacedMutatingPolicy(nmpol)
	default:
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.V(2).Info("policy deleted", "uid", p.GetUID(), "kind", p.GetKind(), "namespace", p.GetNamespace(), "name", p.GetName())
}

func (pc *policyController) enqueuePolicy(policy engineapi.GenericPolicy) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	if policy.AsGeneratingPolicy() != nil {
		pc.queue.Add("gpol/" + key)
	} else if policy.AsNamespacedGeneratingPolicy() != nil {
		pc.queue.Add("ngpol/" + key)
	} else if policy.AsMutatingPolicy() != nil {
		pc.queue.Add("mpol/" + key)
	} else if policy.AsNamespacedMutatingPolicy() != nil {
		pc.queue.Add("nmpol/" + key)
	}
}

// Run begins watching and syncing.
func (pc *policyController) Run(ctx context.Context, workers int) {
	logger := pc.log

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	logger.V(2).Info("starting")
	defer logger.V(2).Info("shutting down")

	if !cache.WaitForNamedCacheSync("PolicyController", ctx.Done(), pc.informersSynced...) {
		return
	}

	if _, err := pc.gpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handler")
		return
	}

	if _, err := pc.ngpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handler")
		return
	}

	if _, err := pc.mpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handler")
		return
	}

	if _, err := pc.nmpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	}); err != nil {
		logger.Error(err, "failed to register event handler")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, pc.worker, time.Second)
	}

	go pc.forceReconciliation(ctx)

	<-ctx.Done()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pc *policyController) worker(ctx context.Context) {
	for pc.processNextWorkItem() {
	}
}

func (pc *policyController) processNextWorkItem() bool {
	key, quit := pc.queue.Get()
	if quit {
		return false
	}
	defer pc.queue.Done(key)
	err := pc.syncPolicy(key.(string))
	pc.handleErr(err, key)

	return true
}

func (pc *policyController) handleErr(err error, key interface{}) {
	logger := pc.log
	if err == nil {
		pc.queue.Forget(key)
		return
	}

	if pc.queue.NumRequeues(key) < maxRetries {
		logger.Error(err, "failed to sync policy", "key", key)
		pc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	logger.V(2).Info("dropping policy out of queue", "key", key)
	pc.queue.Forget(key)
}

func (pc *policyController) syncPolicy(key string) error {
	logger := pc.log.WithName("syncPolicy")
	startTime := time.Now()
	logger.V(4).Info("started syncing policy", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing policy", "key", key, "processingTime", time.Since(startTime).String())
	}()

	parts := strings.SplitN(key, "/", 2)
	polType := parts[0]
	polName := parts[1]
	var errs []error
	switch polType {
	case "gpol":
		gpol, err := pc.gpolLister.Get(polName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		// create UR on policy events to update/generate downstream resources
		if gpol.Spec.SynchronizationEnabled() {
			logger.V(4).Info("creating UR on generating policy events", "name", gpol.GetName())
			if err := pc.createURForGeneratingPolicy(gpol); err != nil {
				logger.Error(err, "failed to create UR on generating policy events", "name", gpol.GetName())
				errs = append(errs, err)
			}
		}
		// generate resources for existing triggers
		if gpol.Spec.GenerateExistingEnabled() {
			logger.V(4).Info("generating resources for existing triggers for generatingpolicy", "name", gpol.GetName())
			if err := pc.handleGenerateExisting(gpol); err != nil {
				logger.Error(err, "failed to create UR for generating policy", "name", gpol.GetName())
				errs = append(errs, err)
			}
		}
	case "ngpol":
		// polName for ngpol is "namespace/name"
		ns, name, err := cache.SplitMetaNamespaceKey(polName)
		if err != nil {
			return err
		}
		ngpol, err := pc.ngpolLister.NamespacedGeneratingPolicies(ns).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		policyKey := ngpol.GetNamespace() + "/" + ngpol.GetName()
		// create UR on policy events to update/generate downstream resources
		if ngpol.Spec.SynchronizationEnabled() {
			logger.V(4).Info("creating UR on namespaced generating policy events", "name", policyKey)
			if err := pc.createURForNamespacedGeneratingPolicy(ngpol); err != nil {
				logger.Error(err, "failed to create UR on namespaced generating policy events", "name", policyKey)
				errs = append(errs, err)
			}
		}
		// generate resources for existing triggers
		if ngpol.Spec.GenerateExistingEnabled() {
			logger.V(4).Info("generating resources for existing triggers for namespacedgeneratingpolicy", "name", policyKey)
			if err := pc.handleNamespacedGenerateExisting(ngpol); err != nil {
				logger.Error(err, "failed to create UR for namespaced generating policy", "name", policyKey)
				errs = append(errs, err)
			}
		}
	case "mpol":
		mpol, err := pc.mpolLister.Get(polName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		// .background only controls background-scan reporting; it never gates
		// mutate-existing execution (https://github.com/kyverno/kyverno/issues/16090).
		if mpol.Spec.MutateExistingEnabled() {
			logger.V(4).Info("creating UR for mutating policy on policy event or background scan", "name", mpol.GetName())
			if err := pc.createURForMutatingPolicy(mpol); err != nil {
				logger.Error(err, "failed to create UR for mutating policy", "name", mpol.GetName())
				errs = append(errs, err)
			}
		}
	case "nmpol":
		ns, name, err := cache.SplitMetaNamespaceKey(polName)
		if err != nil {
			return err
		}
		nmpol, err := pc.nmpolLister.NamespacedMutatingPolicies(ns).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if nmpol.Spec.MutateExistingEnabled() {
			policyKey := nmpol.GetNamespace() + "/" + nmpol.GetName()
			logger.V(4).Info("creating UR for namespaced mutating policy on policy event or background scan", "name", policyKey)
			if err := pc.createURForNamespacedMutatingPolicy(nmpol); err != nil {
				logger.Error(err, "failed to create UR for namespaced mutating policy", "name", policyKey)
				errs = append(errs, err)
			}
		}
	}
	return multierr.Combine(errs...)
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *policyController) forceReconciliation(ctx context.Context) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			logger.V(3).Info("reconciling generate and mutateExisting policies", "scan interval", pc.reconcilePeriod.String())
			pc.requeuePolicies()

		case <-ctx.Done():
			return
		}
	}
}

func (pc *policyController) requeuePolicies() {
	logger := pc.log.WithName("requeuePolicies")
	if gpols, err := pc.gpolLister.List(labels.Everything()); err == nil {
		for _, gpol := range gpols {
			pc.enqueuePolicy(engineapi.NewGeneratingPolicy(gpol))
		}
	} else {
		logger.Error(err, "unable to list GeneratingPolicies")
	}
	if ngpols, err := pc.ngpolLister.List(labels.Everything()); err == nil {
		for _, ngpol := range ngpols {
			pc.enqueuePolicy(engineapi.NewNamespacedGeneratingPolicy(ngpol))
		}
	} else {
		logger.Error(err, "unable to list NamespacedGeneratingPolicies")
	}
	if mpols, err := pc.mpolLister.List(labels.Everything()); err == nil {
		for _, mpol := range mpols {
			// .background never gates mutate-existing execution; it only controls reporting.
			if mpol.Spec.MutateExistingEnabled() &&
				(mpol.Spec.TargetMatchConstraints == nil || mpol.Spec.TargetMatchConstraints.Expression == "") {
				pc.enqueuePolicy(engineapi.NewMutatingPolicy(mpol))
			}
		}
	} else {
		logger.Error(err, "unable to list MutatingPolicies")
	}
	if nmpols, err := pc.nmpolLister.List(labels.Everything()); err == nil {
		for _, nmpol := range nmpols {
			if nmpol.Spec.MutateExistingEnabled() &&
				(nmpol.Spec.TargetMatchConstraints == nil || nmpol.Spec.TargetMatchConstraints.Expression == "") {
				pc.enqueuePolicy(engineapi.NewNamespacedMutatingPolicy(nmpol))
			}
		}
	} else {
		logger.Error(err, "unable to list NamespacedMutatingPolicies")
	}
}
