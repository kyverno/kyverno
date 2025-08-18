package policy

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policiesv1alpha1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	backgroundcommon "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/gpol"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov2informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v2"
	policiesv1alpha1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/policies.kyverno.io/v1alpha1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov2listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v2"
	policiesv1alpha1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	datautils "github.com/kyverno/kyverno/pkg/utils/data"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	"github.com/kyverno/kyverno/pkg/utils/generator"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
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
	engine        engineapi.Engine

	pInformer    kyvernov1informers.ClusterPolicyInformer
	npInformer   kyvernov1informers.PolicyInformer
	gpolInformer policiesv1alpha1informers.GeneratingPolicyInformer

	eventGen      event.Interface
	eventRecorder events.EventRecorder

	// Policies that need to be synced
	queue workqueue.TypedRateLimitingInterface[any]

	// pLister can list/get policy from the shared informer's store
	pLister kyvernov1listers.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernov1listers.PolicyLister

	// gpolLister can list/get generating policy from the shared informer's store
	gpolLister policiesv1alpha1listers.GeneratingPolicyLister

	// urLister can list/get update request from the shared informer's store
	urLister kyvernov2listers.UpdateRequestLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister corev1listers.NamespaceLister

	informersSynced []cache.InformerSynced

	// helpers to validate against current loaded configuration
	configuration config.Configuration

	reconcilePeriod time.Duration

	log logr.Logger

	metricsConfig metrics.MetricsConfigManager

	jp jmespath.Interface

	urGenerator generator.UpdateRequestGenerator

	watchManager *gpol.WatchManager

	// mapper
	restMapper meta.RESTMapper
}

// NewPolicyController create a new PolicyController
func NewPolicyController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	engine engineapi.Engine,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	gpolInformer policiesv1alpha1informers.GeneratingPolicyInformer,
	urInformer kyvernov2informers.UpdateRequestInformer,
	configuration config.Configuration,
	eventGen event.Interface,
	namespaces corev1informers.NamespaceInformer,
	log logr.Logger,
	reconcilePeriod time.Duration,
	metricsConfig metrics.MetricsConfigManager,
	jp jmespath.Interface,
	urGenerator generator.UpdateRequestGenerator,
	watchManager *gpol.WatchManager,
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
		engine:        engine,
		pInformer:     pInformer,
		npInformer:    npInformer,
		gpolInformer:  gpolInformer,
		eventGen:      eventGen,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, "policy_controller"),
		queue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[any](),
			workqueue.TypedRateLimitingQueueConfig[any]{Name: "policy"},
		),
		configuration:   configuration,
		reconcilePeriod: reconcilePeriod,
		metricsConfig:   metricsConfig,
		log:             log,
		jp:              jp,
		urGenerator:     urGenerator,
		watchManager:    watchManager,
	}
	apiGroupResources, _ := restmapper.GetAPIGroupResources(client.GetKubeClient().Discovery())
	pc.restMapper = restmapper.NewDiscoveryRESTMapper(apiGroupResources)

	pc.pLister = pInformer.Lister()
	pc.npLister = npInformer.Lister()
	pc.nsLister = namespaces.Lister()
	pc.urLister = urInformer.Lister()
	pc.gpolLister = gpolInformer.Lister()

	pc.informersSynced = []cache.InformerSynced{pInformer.Informer().HasSynced, npInformer.Informer().HasSynced, gpolInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaces.Informer().HasSynced}

	return &pc, nil
}

func (pc *policyController) canBackgroundProcess(p kyvernov1.PolicyInterface) bool {
	logger := pc.log.WithValues("policy", p.GetName())
	if !p.GetSpec().HasGenerate() && !p.GetSpec().HasMutateExisting() {
		logger.V(4).Info("policy does not have background rules for reconciliation")
		return false
	}

	if err := policyvalidation.ValidateVariables(p, true); err != nil {
		logger.V(4).Info("policy cannot be processed in the background")
		return false
	}

	if p.GetSpec().HasMutateExisting() {
		val := os.Getenv("BACKGROUND_SCAN_INTERVAL")
		interval, err := time.ParseDuration(val)
		if err != nil {
			logger.V(4).Info("The BACKGROUND_SCAN_INTERVAL env variable is not set, therefore the default interval of 1h will be used.", "msg", err.Error())
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
	policy := castPolicy(obj)
	logger.V(2).Info("policy created", "uid", policy.GetUID(), "kind", policy.GetKind(), "namespace", policy.GetNamespace(), "name", policy.GetName())

	if kpol := policy.AsKyvernoPolicy(); kpol != nil {
		if !pc.canBackgroundProcess(kpol) {
			return
		}
	}

	logger.V(4).Info("queuing policy for background processing", "name", policy.GetName())
	pc.enqueuePolicy(policy)
}

func (pc *policyController) updatePolicy(old, new interface{}) {
	logger := pc.log
	oldPolicy := castPolicy(old)
	newPolicy := castPolicy(new)

	oldkpol := oldPolicy.AsKyvernoPolicy()
	newkpol := newPolicy.AsKyvernoPolicy()
	if oldkpol != nil && newkpol != nil {
		if !pc.canBackgroundProcess(newkpol) {
			return
		}

		if datautils.DeepEqual(oldkpol.GetSpec(), newkpol.GetSpec()) {
			return
		}

		if deleted, ok, selector := ruleChange(oldkpol, newkpol); ok {
			err := pc.createURForDownstreamDeletion(deleted)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to create UR on rule deletion, clean up downstream resource may be failed: %v", err))
			}
		} else {
			pc.unlabelDownstream(selector)
		}
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

	logger.V(2).Info("updating policy", "name", oldPolicy.GetName())
	pc.enqueuePolicy(newPolicy)
}

func (pc *policyController) deletePolicy(obj interface{}) {
	logger := pc.log
	var p engineapi.GenericPolicy

	switch kubeutils.GetObjectWithTombstone(obj).(type) {
	case *kyvernov1.ClusterPolicy:
		cpol := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
		err := pc.createURForDownstreamDeletion(cpol)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to create UR on policy deletion, clean up downstream resource may be failed: %v", err))
		}
		p = engineapi.NewKyvernoPolicy(cpol)
	case *kyvernov1.Policy:
		pol := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
		err := pc.createURForDownstreamDeletion(pol)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to create UR on policy deletion, clean up downstream resource may be failed: %v", err))
		}
		p = engineapi.NewKyvernoPolicy(pol)
	case *policiesv1alpha1.GeneratingPolicy:
		gpol := kubeutils.GetObjectWithTombstone(obj).(*policiesv1alpha1.GeneratingPolicy)
		if gpol.Spec.OrphanDownstreamOnPolicyDeleteEnabled() {
			pc.watchManager.RemoveWatchersForPolicy(gpol.GetName(), false)
		} else {
			pc.watchManager.RemoveWatchersForPolicy(gpol.GetName(), true)
		}
		p = engineapi.NewGeneratingPolicy(gpol)
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
	if policy.AsKyvernoPolicy() != nil {
		pc.queue.Add("kpol/" + key)
	} else if policy.AsGeneratingPolicy() != nil {
		pc.queue.Add("gpol/" + key)
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

	_, _ = pc.gpolInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
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

	parts := strings.SplitN(key, "/", 2)
	polType := parts[0]
	polName := parts[1]
	if polType == "kpol" {
		policy, err := pc.getPolicy(polName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		} else {
			err = pc.handleMutate(polName, policy)
			if err != nil {
				logger.Error(err, "failed to updateUR on mutate policy update")
			}

			err = pc.handleGenerate(polName, policy)
			if err != nil {
				logger.Error(err, "failed to updateUR on generate policy update")
			}
		}
	} else if polType == "gpol" {
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
			err := pc.createURForGeneratingPolicy(gpol)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to create UR on generating policy events %s: %v", gpol.GetName(), err))
			}
		}
		// generate resources for existing triggers
		if gpol.Spec.GenerateExistingEnabled() {
			logger.V(4).Info("generating resources for existing triggers for generatingpolicy", "name", gpol.GetName())
			err := pc.handleGenerateExisting(gpol)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("failed to create UR for generating policy %s: %v", gpol.GetName(), err))
			}
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
			logger.V(3).Info("reconciling generate and mutateExisting policies", "scan interval", pc.reconcilePeriod.String())
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
			pc.enqueuePolicy(engineapi.NewKyvernoPolicy(cpol))
		}
	} else {
		logger.Error(err, "unable to list ClusterPolicies")
	}
	if pols, err := pc.npLister.Policies(metav1.NamespaceAll).List(labels.Everything()); err == nil {
		for _, p := range pols {
			if !pc.canBackgroundProcess(p) {
				continue
			}
			pc.enqueuePolicy(engineapi.NewKyvernoPolicy(p))
		}
	} else {
		logger.Error(err, "unable to list Policies")
	}
	if gpols, err := pc.gpolLister.List(labels.Everything()); err == nil {
		for _, gpol := range gpols {
			pc.enqueuePolicy(engineapi.NewGeneratingPolicy(gpol))
		}
	} else {
		logger.Error(err, "unable to list GeneratingPolicies")
	}
}

func (pc *policyController) handleUpdateRequest(ur *kyvernov2.UpdateRequest, triggerResource *unstructured.Unstructured, ruleName string, policy kyvernov1.PolicyInterface) (skip bool, err error) {
	namespaceLabels, err := engineutils.GetNamespaceSelectorsFromNamespaceLister(triggerResource.GetKind(), triggerResource.GetNamespace(), pc.nsLister, []kyvernov1.PolicyInterface{policy}, pc.log)
	if err != nil {
		return false, fmt.Errorf("failed to get namespace labels for rule %s: %w", ruleName, err)
	}

	policyContext, err := backgroundcommon.NewBackgroundContext(pc.log, pc.client, ur.Spec.Context, policy, triggerResource, pc.configuration, pc.jp, namespaceLabels)
	if err != nil {
		return false, fmt.Errorf("failed to build policy context for rule %s: %w", ruleName, err)
	}

	engineResponse := pc.engine.ApplyBackgroundChecks(context.TODO(), policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		return true, nil
	}

	for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
		if ruleResponse.Status() != engineapi.RuleStatusPass {
			pc.log.V(4).Info("skip creating URs on policy update", "policy", policy.GetName(), "rule", ruleName, "rule.Status", ruleResponse.Status())
			continue
		}

		if ruleResponse.Name() != ur.Spec.GetRuleName() {
			continue
		}

		pc.log.V(2).Info("creating new UR for generate")
		created, err := pc.urGenerator.Generate(context.TODO(), pc.kyvernoClient, ur, pc.log)
		if err != nil {
			return false, err
		}
		if created == nil {
			continue
		}
		updated := created.DeepCopy()
		updated.Status.State = kyvernov2.Pending
		_, err = pc.kyvernoClient.KyvernoV2().UpdateRequests(config.KyvernoNamespace()).UpdateStatus(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
	}
	return false, err
}

func getTriggers(client dclient.Interface, rule kyvernov1.Rule, isNamespacedPolicy bool, policyNamespace string, log logr.Logger) []*unstructured.Unstructured {
	var resources []*unstructured.Unstructured

	appendResources := func(match kyvernov1.ResourceDescription) {
		resources = append(resources, getResources(client, policyNamespace, isNamespacedPolicy, match, log)...)
	}

	if !rule.MatchResources.ResourceDescription.IsEmpty() {
		appendResources(rule.MatchResources.ResourceDescription)
	}

	for _, any := range rule.MatchResources.Any {
		appendResources(any.ResourceDescription)
	}

	for _, all := range rule.MatchResources.All {
		appendResources(all.ResourceDescription)
	}

	return resources
}

func getResources(client dclient.Interface, policyNs string, isNamespacedPolicy bool, match kyvernov1.ResourceDescription, log logr.Logger) []*unstructured.Unstructured {
	var items []*unstructured.Unstructured

	for _, kind := range match.Kinds {
		group, version, kind, _ := kubeutils.ParseKindSelector(kind)

		namespace := ""
		if isNamespacedPolicy {
			namespace = policyNs
		}

		groupVersion := ""
		if group != "*" && version != "*" {
			groupVersion = group + "/" + version
		} else if version != "*" {
			groupVersion = version
		}

		resources, err := client.ListResource(context.TODO(), groupVersion, kind, namespace, match.Selector)
		if err != nil {
			log.Error(err, "failed to list matched resource")
			continue
		}

		for i, res := range resources.Items {
			if !resourceMatches(match, res, isNamespacedPolicy) {
				continue
			}
			items = append(items, &resources.Items[i])
		}
	}
	return items
}
