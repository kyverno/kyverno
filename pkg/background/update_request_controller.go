package background

import (
	"context"
	"fmt"
	"time"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/gpol"
	"github.com/kyverno/kyverno/pkg/background/mpol"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	gpolengine "github.com/kyverno/kyverno/pkg/cel/policies/gpol/engine"
	mpolengine "github.com/kyverno/kyverno/pkg/cel/policies/mpol/engine"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	reportutils "github.com/kyverno/kyverno/pkg/utils/report"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
	// emptyStateRecoveryDelay waits out the creator's initial Pending write.
	// Admission retries that write for up to 3s (generator.applyResource) and
	// may create another request if it keeps failing. Recovering sooner races
	// that write: the creator's UpdateStatus then conflicts, and the caller
	// can submit a second request for work this one is about to run.
	emptyStateRecoveryDelay = 5 * time.Second
)

type Controller interface {
	// Run starts workers
	Run(context.Context, int)
}

// controller manages the life-cycle of UpdateRequests (UR v2) for CEL
// GeneratingPolicy and MutatingPolicy, and drops legacy kyverno.io/v1
// mutate/generate URs left over from pre-1.20 releases.
type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface

	// listers
	urLister kyvernov2listers.UpdateRequestNamespaceLister

	informersSynced []cache.InformerSynced

	// queue
	queue workqueue.TypedRateLimitingInterface[any]

	context      libs.Context
	gpolEngine   gpolengine.Engine
	gpolProvider gpolengine.Provider
	watchManager *gpol.WatchManager

	mpolEngine    mpolengine.Engine
	restMapper    meta.RESTMapper
	eventGen      event.Interface
	configuration config.Configuration
}

// NewController returns an instance of the UpdateRequest controller
func NewController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	urInformer kyvernov2informers.UpdateRequestInformer,
	context libs.Context,
	gpolEngine gpolengine.Engine,
	gpolProvider gpolengine.Provider,
	watchManager *gpol.WatchManager,
	mpolEngine mpolengine.Engine,
	restMapper meta.RESTMapper,
	eventGen event.Interface,
	configuration config.Configuration,
	reportsConfig reportutils.ReportingConfiguration,
) Controller {
	urLister := urInformer.Lister().UpdateRequests(config.KyvernoNamespace())
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		urLister:      urLister,
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "background"},
		),
		context:       context,
		gpolEngine:    gpolEngine,
		gpolProvider:  gpolProvider,
		watchManager:  watchManager,
		mpolEngine:    mpolEngine,
		restMapper:    restMapper,
		eventGen:      eventGen,
		configuration: configuration,
	}
	_, _ = urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addUR,
		UpdateFunc: c.updateUR,
	})

	c.informersSynced = []cache.InformerSynced{urInformer.Informer().HasSynced}

	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger.V(4).Info("starting")
	defer logger.V(4).Info("shutting down")

	if !cache.WaitForNamedCacheSync("background", ctx.Done(), c.informersSynced...) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.worker, time.Second)
	}

	<-ctx.Done()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *controller) worker(ctx context.Context) {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}

	defer c.queue.Done(key)
	err := c.syncUpdateRequest(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *controller) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if apierrors.IsNotFound(err) {
		c.queue.Forget(key)
		logger.V(4).Info("Dropping update request from the queue", "key", key, "error", err.Error())
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		logger.V(3).Info("retrying update request", "key", key, "error", err.Error())
		c.queue.AddRateLimited(key)
		return
	}

	logger.Error(err, "failed to process update request", "key", key)
	c.queue.Forget(key)
}

func (c *controller) syncUpdateRequest(key string) error {
	startTime := time.Now()
	logger.V(4).Info("started sync", "key", key, "startTime", startTime)
	_, urName, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	ur, err := c.urLister.Get(urName)
	if err != nil {
		return err
	}

	// Deep-copy otherwise we are mutating our cache.
	ur = ur.DeepCopy()
	switch ur.Spec.GetRequestType() {
	case kyvernov2.Mutate, kyvernov2.Generate:
		return c.dropLegacyUpdateRequest(ur)
	}

	if ur.Status.State == kyvernov2.Pending {
		if err := c.processUR(ur); err != nil {
			return fmt.Errorf("failed to process UR %s: %v", key, err)
		}
	}

	urStatus, err := c.reconcileURStatus(ur)
	if err != nil {
		return err
	}

	logger.V(4).Info("synced update request", "key", key, "processingTime", time.Since(startTime).String(), "ur status", urStatus)
	return nil
}

func (c *controller) enqueueUpdateRequest(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract name")
		return
	}
	logger.V(5).Info("enqueued update request", "ur", key)
	c.queue.Add(key)
}

func (c *controller) addUR(obj interface{}) {
	ur := obj.(*kyvernov2.UpdateRequest)
	c.enqueueUpdateRequest(ur)
}

func (c *controller) updateUR(_, cur interface{}) {
	curUr := cur.(*kyvernov2.UpdateRequest)
	switch curUr.Spec.GetRequestType() {
	case kyvernov2.Mutate, kyvernov2.Generate:
		// dropLegacyUpdateRequest runs before the Pending check in
		// syncUpdateRequest and deletes regardless of state, so a legacy UR
		// must always re-enqueue here too - including Skip/Completed, which
		// the state filter below would otherwise drop. Without this, a
		// legacy UR whose delete attempt exhausted maxRetries would only be
		// retried on controller restart, since resyncs arrive as Update
		// events and would hit that filter.
		c.enqueueUpdateRequest(curUr)
		return
	}
	if curUr.Status.State == kyvernov2.Skip || curUr.Status.State == kyvernov2.Completed {
		return
	}
	// A retry (a Failed UR, or a Pending one with RetryCount > 0) backs off
	// instead of an immediate re-enqueue.
	if curUr.Status.State == kyvernov2.Failed || curUr.Status.RetryCount > 0 {
		key, err := cache.MetaNamespaceKeyFunc(curUr)
		if err != nil {
			logger.Error(err, "failed to extract name")
			return
		}
		delay := defaultRetryBackoff(curUr.Status.RetryCount)
		logger.V(3).Info("retrying update request with backoff", "key", key, "retryCount", curUr.Status.RetryCount, "delay", delay)
		c.queue.AddAfter(key, delay)
		return
	}
	c.enqueueUpdateRequest(curUr)
}

// emptyStateRecoveryWait reports how long to leave an empty UpdateRequest
// alone. A zero creation timestamp has no in-flight writer to wait for.
// A timestamp in the future (clock skew) waits the full delay.
func emptyStateRecoveryWait(created, now time.Time) time.Duration {
	if created.IsZero() {
		return 0
	}
	age := now.Sub(created)
	if age < 0 {
		return emptyStateRecoveryDelay
	}
	if age >= emptyStateRecoveryDelay {
		return 0
	}
	return emptyStateRecoveryDelay - age
}

func (c *controller) requeueUpdateRequest(obj interface{}, delay time.Duration) {
	if c.queue == nil || delay <= 0 {
		return
	}
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Error(err, "failed to extract name")
		return
	}
	logger.V(4).Info("delaying empty update request recovery", "ur", key, "delay", delay)
	c.queue.AddAfter(key, delay)
}

// defaultRetryBackoff paces UR retries (300ms, 600ms, 1.2s..., capped at
// 10s) against retryOrDeleteOnFailure's RetryCount>3 delete threshold.
func defaultRetryBackoff(retryCount int) time.Duration {
	const (
		base     = 300 * time.Millisecond
		maxDelay = 10 * time.Second
	)
	d := base
	for i := 1; i < retryCount; i++ {
		d *= 2
		if d >= maxDelay {
			return maxDelay
		}
	}
	return d
}

func (c *controller) processUR(ur *kyvernov2.UpdateRequest) error {
	statusControl := common.NewStatusControl(c.kyvernoClient, c.urLister)
	switch t := ur.Spec.GetRequestType(); t {
	case kyvernov2.CELGenerate:
		ctrl := gpol.NewCELGenerateController(c.client, c.kyvernoClient, c.context, c.gpolEngine, c.gpolProvider, c.watchManager, statusControl, c.eventGen, logger, c.configuration)
		return ctrl.ProcessUR(ur)
	case kyvernov2.CELMutate:
		processor := mpol.NewProcessor(c.client, c.kyvernoClient, c.mpolEngine, c.restMapper, c.context, statusControl, c.eventGen, c.configuration)
		return processor.Process(ur)
	default:
		_, err := statusControl.Failed(ur.GetName(), fmt.Sprintf("unsupported update request type %q", t), nil)
		return err
	}
}

func (c *controller) reconcileURStatus(ur *kyvernov2.UpdateRequest) (kyvernov2.UpdateRequestState, error) {
	return c.reconcileURStatusAt(ur, time.Now())
}

func (c *controller) reconcileURStatusAt(ur *kyvernov2.UpdateRequest, now time.Time) (kyvernov2.UpdateRequestState, error) {
	new, err := c.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), ur.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// UR was already deleted (e.g., by another controller or manually), nothing to do
			logger.V(4).Info("update request not found, skipping reconciliation", "name", ur.GetName())
			return kyvernov2.Skip, nil
		}
		// Return error to force retry - do NOT fall back to stale data as this can cause
		// Completed URs to never be deleted if the informer event handler ignores them
		return "", fmt.Errorf("failed to fetch latest UR status for %s: %w", ur.GetName(), err)
	}

	var errUpdate error
	switch new.Status.State {
	case kyvernov2.Completed:
		errUpdate = c.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	case kyvernov2.Failed:
		new.Status.State = kyvernov2.Pending
		_, errUpdate = c.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{})
	case "":
		// Create and the initial Pending write are separate calls. A failed
		// status write leaves the request empty. Sync only runs Pending, and
		// the cases above only repair Failed and Completed, so the request
		// would sit forever and still count toward updateRequestThreshold.
		// Wait out the creator's retry window, then set Pending. The status
		// update requeues the normal processing path. This uses the object
		// just read, so a newer Pending, Skip, Failed, or Completed state is
		// handled by its own case instead of this one.
		if wait := emptyStateRecoveryWait(new.CreationTimestamp.Time, now); wait > 0 {
			c.requeueUpdateRequest(new, wait)
			return new.Status.State, nil
		}
		new.Status.State = kyvernov2.Pending
		_, errUpdate = c.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{})
	}
	return new.Status.State, errUpdate
}
