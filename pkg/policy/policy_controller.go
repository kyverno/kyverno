package policy

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	backgroundcommon "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	engine        engineapi.Engine

	pInformer  kyvernov1informers.ClusterPolicyInformer
	npInformer kyvernov1informers.PolicyInformer

	eventGen      event.Interface
	eventRecorder events.EventRecorder

	// Policies that need to be synced
	queue workqueue.RateLimitingInterface

	// pLister can list/get policy from the shared informer's store
	pLister kyvernov1listers.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernov1listers.PolicyLister

	// urLister can list/get update request from the shared informer's store
	urLister kyvernov1beta1listers.UpdateRequestLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	// helpers to validate against current loaded configuration
	configuration config.Configuration

	reconcilePeriod time.Duration

	log logr.Logger

	metricsConfig metrics.MetricsConfigManager

	jp jmespath.Interface
}

// NewPolicyController create a new PolicyController
func NewPolicyController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	engine engineapi.Engine,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	configuration config.Configuration,
	eventGen event.Interface,
	namespaces corev1informers.NamespaceInformer,
	log logr.Logger,
	reconcilePeriod time.Duration,
	metricsConfig metrics.MetricsConfigManager,
	jp jmespath.Interface,
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
		client:          client,
		kyvernoClient:   kyvernoClient,
		engine:          engine,
		pInformer:       pInformer,
		npInformer:      npInformer,
		eventGen:        eventGen,
		eventRecorder:   eventBroadcaster.NewRecorder(scheme.Scheme, "policy_controller"),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		configuration:   configuration,
		reconcilePeriod: reconcilePeriod,
		metricsConfig:   metricsConfig,
		log:             log,
		jp:              jp,
	}

	pc.pLister = pInformer.Lister()
	pc.npLister = npInformer.Lister()
	pc.nsLister = namespaces.Lister()
	pc.urLister = urInformer.Lister()

	pc.informersSynced = []cache.InformerSynced{pInformer.Informer().HasSynced, npInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaces.Informer().HasSynced}

	return &pc, nil
}

func (pc *policyController) canBackgroundProcess(p kyvernov1.PolicyInterface) bool {
	logger := pc.log.WithValues("policy", p.GetName())
	if !p.GetSpec().HasGenerate() && !p.GetSpec().IsMutateExisting() {
		logger.V(4).Info("policy does not have background rules for reconciliation")
		return false
	}

	if err := policyvalidation.ValidateVariables(p, true); err != nil {
		logger.V(4).Info("policy cannot be processed in the background")
		return false
	}

	if p.GetSpec().IsMutateExisting() {
		val := os.Getenv("BACKGROUND_SCAN_INTERVAL")
		interval, err := time.ParseDuration(val)
		if err != nil {
			logger.V(4).Info("failed to parse BACKGROUND_SCAN_INTERVAL env variable, falling to default 1h", "msg", err.Error())
			interval = time.Hour
		}
		if p.GetCreationTimestamp().Add(interval).After(time.Now()) {
			return p.GetSpec().GetMutateExistingOnPolicyUpdate()
		}
	}

	return true
}

func (pc *policyController) addPolicy(obj interface{}) {
	logger := pc.log
	p := castPolicy(obj)
	logger.Info("policy created", "uid", p.GetUID(), "kind", p.GetKind(), "namespace", p.GetNamespace(), "name", p.GetName())

	if !pc.canBackgroundProcess(p) {
		return
	}

	logger.V(4).Info("queuing policy for background processing", "name", p.GetName())
	pc.enqueuePolicy(p)
}

func (pc *policyController) updatePolicy(old, cur interface{}) {
	logger := pc.log
	oldP := castPolicy(old)
	curP := castPolicy(cur)
	if !pc.canBackgroundProcess(curP) {
		return
	}

	if datautils.DeepEqual(oldP.GetSpec(), curP.GetSpec()) {
		return
	}

	logger.V(2).Info("updating policy", "name", oldP.GetName())
	if deleted, ok := ruleDeletion(oldP, curP); ok {
		err := pc.createURForDownstreamDeletion(deleted)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to create UR on rule deletion, clean up downstream resource may be failed: %v", err))
		}
	}

	pc.enqueuePolicy(curP)
}

func (pc *policyController) deletePolicy(obj interface{}) {
	logger := pc.log
	var p kyvernov1.PolicyInterface

	switch kubeutils.GetObjectWithTombstone(obj).(type) {
	case *kyvernov1.ClusterPolicy:
		p = kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	case *kyvernov1.Policy:
		p = kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	default:
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.Info("policy deleted", "uid", p.GetUID(), "kind", p.GetKind(), "namespace", p.GetNamespace(), "name", p.GetName())
	err := pc.createURForDownstreamDeletion(p)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("failed to create UR on policy deletion, clean up downstream resource may be failed: %v", err))
	}
}

func (pc *policyController) enqueuePolicy(policy kyvernov1.PolicyInterface) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *policyController) Run(ctx context.Context, workers int) {
	logger := pc.log

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("PolicyController", ctx.Done(), pc.informersSynced...) {
		return
	}

	_, _ = pc.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	_, _ = pc.npInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

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

	policy, err := pc.getPolicy(key)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	} else {
		err = pc.handleMutate(key, policy)
		if err != nil {
			logger.Error(err, "failed to updateUR on mutate policy update")
		}

		err = pc.handleGenerate(key, policy)
		if err != nil {
			logger.Error(err, "failed to updateUR on generate policy update")
		}
	}
	return nil
}

func (pc *policyController) getPolicy(key string) (kyvernov1.PolicyInterface, error) {
	if ns, name, err := cache.SplitMetaNamespaceKey(key); err != nil {
		pc.log.Error(err, "failed to parse policy name", "policyName", key)
		return nil, err
	} else {
		isNamespacedPolicy := ns != ""
		if !isNamespacedPolicy {
			return pc.pLister.Get(name)
		}
		return pc.npLister.Policies(ns).Get(name)
	}
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *policyController) forceReconciliation(ctx context.Context) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)

	for {
		select {
		case <-ticker.C:
			logger.Info("reconciling generate and mutateExisting policies", "scan interval", pc.reconcilePeriod.String())
			pc.requeuePolicies()

		case <-ctx.Done():
			return
		}
	}
}

func (pc *policyController) requeuePolicies() {
	logger := pc.log.WithName("requeuePolicies")
	if cpols, err := pc.pLister.List(labels.Everything()); err == nil {
		for _, cpol := range cpols {
			if !pc.canBackgroundProcess(cpol) {
				continue
			}
			pc.enqueuePolicy(cpol)
		}
	} else {
		logger.Error(err, "unable to list ClusterPolicies")
	}
	if pols, err := pc.npLister.Policies(metav1.NamespaceAll).List(labels.Everything()); err == nil {
		for _, p := range pols {
			if !pc.canBackgroundProcess(p) {
				continue
			}
			pc.enqueuePolicy(p)
		}
	} else {
		logger.Error(err, "unable to list Policies")
	}
}

func (pc *policyController) handleUpdateRequest(ur *kyvernov1beta1.UpdateRequest, triggerResource *unstructured.Unstructured, rule kyvernov1.Rule, policy kyvernov1.PolicyInterface) (skip bool, err error) {
	namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(triggerResource.GetKind(), triggerResource.GetNamespace(), pc.nsLister, pc.log)
	policyContext, err := backgroundcommon.NewBackgroundContext(pc.log, pc.client, ur, policy, triggerResource, pc.configuration, pc.jp, namespaceLabels)
	if err != nil {
		return false, fmt.Errorf("failed to build policy context for rule %s: %w", rule.Name, err)
	}

	engineResponse := pc.engine.ApplyBackgroundChecks(context.TODO(), policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		return true, nil
	}

	for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
		if ruleResponse.Status() != engineapi.RuleStatusPass {
			pc.log.V(4).Info("skip creating URs on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule.Status", ruleResponse.Status())
			continue
		}

		pc.log.V(2).Info("creating new UR for generate")
		created, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
		updated := created.DeepCopy()
		updated.Status.State = kyvernov1beta1.Pending
		_, err = pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
	}
	return false, err
}

func generateTriggers(client dclient.Interface, rule kyvernov1.Rule, log logr.Logger) []*unstructured.Unstructured {
	list := &unstructured.UnstructuredList{}

	kinds := fetchUniqueKinds(rule)

	for _, kind := range kinds {
		mlist, err := client.ListResource(context.TODO(), "", kind, "", rule.MatchResources.Selector)
		if err != nil {
			log.Error(err, "failed to list matched resource")
			continue
		}
		list.Items = append(list.Items, mlist.Items...)
	}
	return convertlist(list.Items)
}
