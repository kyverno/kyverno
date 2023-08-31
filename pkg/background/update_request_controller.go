package background

import (
	"context"
	"fmt"
	"time"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	common "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate"
	"github.com/kyverno/kyverno/pkg/background/mutate"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	maxRetries = 10
)

type Controller interface {
	// Run starts workers
	Run(context.Context, int)
}

// controller manages the life-cycle for Generate-Requests and applies generate rule
type controller struct {
	// clients
	client        dclient.Interface
	kyvernoClient versioned.Interface
	engine        engineapi.Engine

	// listers
	cpolLister kyvernov1listers.ClusterPolicyLister
	polLister  kyvernov1listers.PolicyLister
	urLister   kyvernov1beta1listers.UpdateRequestNamespaceLister
	nsLister   corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	// queue
	queue workqueue.RateLimitingInterface

	eventGen      event.Interface
	configuration config.Configuration
	jp            jmespath.Interface
}

// NewController returns an instance of the Generate-Request Controller
func NewController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	engine engineapi.Engine,
	cpolInformer kyvernov1informers.ClusterPolicyInformer,
	polInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	namespaceInformer corev1informers.NamespaceInformer,
	eventGen event.Interface,
	configuration config.Configuration,
	jp jmespath.Interface,
) Controller {
	urLister := urInformer.Lister().UpdateRequests(config.KyvernoNamespace())
	c := controller{
		client:        client,
		kyvernoClient: kyvernoClient,
		engine:        engine,
		cpolLister:    cpolInformer.Lister(),
		polLister:     polInformer.Lister(),
		urLister:      urLister,
		nsLister:      namespaceInformer.Lister(),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "background"),
		eventGen:      eventGen,
		configuration: configuration,
		jp:            jp,
	}
	_, _ = urInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addUR,
		UpdateFunc: c.updateUR,
	})

	c.informersSynced = []cache.InformerSynced{cpolInformer.Informer().HasSynced, polInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaceInformer.Informer().HasSynced}

	return &c
}

func (c *controller) Run(ctx context.Context, workers int) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

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
		c.queue.AddAfter(key, time.Second)
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
	if _, err := c.getPolicy(ur.Spec.Policy); err != nil && apierrors.IsNotFound(err) {
		if ur.Spec.GetRequestType() == kyvernov1beta1.Mutate {
			return c.handleMutatePolicyAbsence(ur)
		}
	}

	if ur.Status.State == kyvernov1beta1.Pending {
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
	ur := obj.(*kyvernov1beta1.UpdateRequest)
	c.enqueueUpdateRequest(ur)
}

func (c *controller) updateUR(_, cur interface{}) {
	curUr := cur.(*kyvernov1beta1.UpdateRequest)
	if curUr.Status.State == kyvernov1beta1.Skip || curUr.Status.State == kyvernov1beta1.Completed {
		return
	}
	c.enqueueUpdateRequest(curUr)
}

func (c *controller) processUR(ur *kyvernov1beta1.UpdateRequest) error {
	statusControl := common.NewStatusControl(c.kyvernoClient, c.urLister)
	switch ur.Spec.GetRequestType() {
	case kyvernov1beta1.Mutate:
		ctrl := mutate.NewMutateExistingController(c.client, c.kyvernoClient, statusControl, c.engine, c.cpolLister, c.polLister, c.nsLister, c.configuration, c.eventGen, logger, c.jp)
		return ctrl.ProcessUR(ur)
	case kyvernov1beta1.Generate:
		ctrl := generate.NewGenerateController(c.client, c.kyvernoClient, statusControl, c.engine, c.cpolLister, c.polLister, c.urLister, c.nsLister, c.configuration, c.eventGen, logger, c.jp)
		return ctrl.ProcessUR(ur)
	}
	return nil
}

func (c *controller) reconcileURStatus(ur *kyvernov1beta1.UpdateRequest) (kyvernov1beta1.UpdateRequestState, error) {
	new, err := c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Get(context.TODO(), ur.GetName(), metav1.GetOptions{})
	if err != nil {
		logger.V(2).Info("cannot fetch latest UR, fallback to the existing one", "reason", err.Error())
		new = ur
	}

	var errUpdate error
	switch new.Status.State {
	case kyvernov1beta1.Completed:
		errUpdate = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Delete(context.TODO(), ur.GetName(), metav1.DeleteOptions{})
	case kyvernov1beta1.Failed:
		new.Status.State = kyvernov1beta1.Pending
		_, errUpdate = c.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{})
	}
	return new.Status.State, errUpdate
}

func (c *controller) getPolicy(key string) (kyvernov1.PolicyInterface, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		return c.cpolLister.Get(name)
	}
	return c.polLister.Policies(namespace).Get(name)
}
