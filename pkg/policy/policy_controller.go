package policy

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/autogen"
	backgroundcommon "github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernov1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernov1beta1informers "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/context/resolvers"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	policyExecutionDuration "github.com/kyverno/kyverno/pkg/metrics/policyexecutionduration"
	policyResults "github.com/kyverno/kyverno/pkg/metrics/policyresults"
	"github.com/kyverno/kyverno/pkg/registryclient"
	engineutils "github.com/kyverno/kyverno/pkg/utils/engine"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	"github.com/kyverno/kyverno/pkg/utils/wildcard"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1informers "k8s.io/client-go/informers/core/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
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

// PolicyController is responsible for synchronizing Policy objects stored
// in the system with the corresponding policy violations
type PolicyController struct {
	client        dclient.Interface
	kyvernoClient versioned.Interface
	rclient       registryclient.Client

	pInformer  kyvernov1informers.ClusterPolicyInformer
	npInformer kyvernov1informers.PolicyInformer

	eventGen      event.Interface
	eventRecorder record.EventRecorder

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

	informerCacheResolvers resolvers.ConfigmapResolver

	informersSynced []cache.InformerSynced

	// Resource manager, manages the mapping for already processed resource
	rm ResourceManager

	// helpers to validate against current loaded configuration
	configHandler config.Configuration

	reconcilePeriod time.Duration

	log logr.Logger

	metricsConfig metrics.MetricsConfigManager
}

// NewPolicyController create a new PolicyController
func NewPolicyController(
	kyvernoClient versioned.Interface,
	client dclient.Interface,
	rclient registryclient.Client,
	pInformer kyvernov1informers.ClusterPolicyInformer,
	npInformer kyvernov1informers.PolicyInformer,
	urInformer kyvernov1beta1informers.UpdateRequestInformer,
	configHandler config.Configuration,
	eventGen event.Interface,
	namespaces corev1informers.NamespaceInformer,
	informerCacheResolvers resolvers.ConfigmapResolver,
	log logr.Logger,
	reconcilePeriod time.Duration,
	metricsConfig metrics.MetricsConfigManager,
) (*PolicyController, error) {
	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.V(5).Info)
	eventInterface := client.GetEventsInterface()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pc := PolicyController{
		client:                 client,
		kyvernoClient:          kyvernoClient,
		rclient:                rclient,
		pInformer:              pInformer,
		npInformer:             npInformer,
		eventGen:               eventGen,
		eventRecorder:          eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "policy_controller"}),
		queue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		configHandler:          configHandler,
		informerCacheResolvers: informerCacheResolvers,
		reconcilePeriod:        reconcilePeriod,
		metricsConfig:          metricsConfig,
		log:                    log,
	}

	pc.pLister = pInformer.Lister()
	pc.npLister = npInformer.Lister()
	pc.nsLister = namespaces.Lister()
	pc.urLister = urInformer.Lister()

	pc.informersSynced = []cache.InformerSynced{pInformer.Informer().HasSynced, npInformer.Informer().HasSynced, urInformer.Informer().HasSynced, namespaces.Informer().HasSynced}
	// resource manager
	// rebuild after 300 seconds/ 5 mins
	pc.rm = NewResourceManager(30)

	return &pc, nil
}

func (pc *PolicyController) canBackgroundProcess(p kyvernov1.PolicyInterface) bool {
	logger := pc.log.WithValues("policy", p.GetName())
	if !p.BackgroundProcessingEnabled() {
		if !p.GetSpec().HasGenerate() && !p.GetSpec().IsMutateExisting() {
			logger.V(4).Info("background processing is disabled")
			return false
		}
	}

	if err := ValidateVariables(p, true); err != nil {
		logger.V(4).Info("policy cannot be processed in the background")
		return false
	}

	return true
}

func (pc *PolicyController) addPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyvernov1.ClusterPolicy)

	logger.Info("policy created", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	if !pc.canBackgroundProcess(p) {
		return
	}

	logger.V(4).Info("queuing policy for background processing", "name", p.Name)
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) updatePolicy(old, cur interface{}) {
	logger := pc.log
	oldP := old.(*kyvernov1.ClusterPolicy)
	curP := cur.(*kyvernov1.ClusterPolicy)

	if !pc.canBackgroundProcess(curP) {
		return
	}

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(2).Info("updating policy", "name", oldP.Name)

	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deletePolicy(obj interface{}) {
	logger := pc.log
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.ClusterPolicy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.Info("policy deleted", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	// do not clean up UR on generate clone (sync=true) policy deletion
	rules := autogen.ComputeRules(p)
	for _, r := range rules {
		clone, sync := r.GetCloneSyncForGenerate()
		if clone && sync {
			return
		}
	}
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) addNsPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyvernov1.Policy)

	logger.Info("policy created", "uid", p.UID, "kind", "Policy", "name", p.Name, "namespaces", p.Namespace)

	if !pc.canBackgroundProcess(p) {
		return
	}
	logger.V(4).Info("queuing policy for background processing", "namespace", p.GetNamespace(), "name", p.GetName())
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) updateNsPolicy(old, cur interface{}) {
	logger := pc.log
	oldP := old.(*kyvernov1.Policy)
	curP := cur.(*kyvernov1.Policy)

	if !pc.canBackgroundProcess(curP) {
		return
	}

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(4).Info("updating namespace policy", "namespace", oldP.Namespace, "name", oldP.Name)

	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deleteNsPolicy(obj interface{}) {
	logger := pc.log
	p, ok := kubeutils.GetObjectWithTombstone(obj).(*kyvernov1.Policy)
	if !ok {
		logger.Info("Failed to get deleted object", "obj", obj)
		return
	}

	logger.Info("policy deleted event", "uid", p.UID, "kind", "Policy", "policy_name", p.Name, "namespaces", p.Namespace)

	pol := p

	// do not clean up UR on generate clone (sync=true) policy deletion
	rules := autogen.ComputeRules(pol)
	for _, r := range rules {
		clone, sync := r.GetCloneSyncForGenerate()
		if clone && sync {
			return
		}
	}
	pc.enqueuePolicy(pol)
}

func (pc *PolicyController) enqueuePolicy(policy kyvernov1.PolicyInterface) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *PolicyController) Run(ctx context.Context, workers int) {
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
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})

	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, pc.worker, time.Second)
	}

	go pc.forceReconciliation(ctx)

	<-ctx.Done()
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pc *PolicyController) worker(ctx context.Context) {
	for pc.processNextWorkItem() {
	}
}

func (pc *PolicyController) processNextWorkItem() bool {
	key, quit := pc.queue.Get()
	if quit {
		return false
	}
	defer pc.queue.Done(key)
	err := pc.syncPolicy(key.(string))
	pc.handleErr(err, key)

	return true
}

func (pc *PolicyController) handleErr(err error, key interface{}) {
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

func (pc *PolicyController) syncPolicy(key string) error {
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
		err = pc.updateUR(key, policy)
		if err != nil {
			logger.Error(err, "failed to updateUR on Policy update")
		}
	}
	pc.processExistingResources(policy)
	return nil
}

func (pc *PolicyController) getPolicy(key string) (kyvernov1.PolicyInterface, error) {
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

func (pc *PolicyController) processExistingResources(policy kyvernov1.PolicyInterface) {
	logger := pc.log.WithValues("policy", policy.GetName())
	logger.V(4).Info("applying policy to existing resources")

	// Parse through all the resources drops the cache after configured rebuild time
	pc.rm.Drop()

	for _, rule := range autogen.ComputeRules(policy) {
		if !rule.HasValidate() && !rule.HasVerifyImages() {
			continue
		}
		matchKinds := rule.MatchResources.GetKinds()
		pc.processExistingKinds(matchKinds, policy, rule, logger)
	}
}

func (pc *PolicyController) applyAndReportPerNamespace(policy kyvernov1.PolicyInterface, kind string, ns string, rule kyvernov1.Rule, logger logr.Logger, metricAlreadyRegistered *bool) {
	rMap := pc.getResourcesPerNamespace(kind, ns, rule, logger)
	excludeAutoGenResources(policy, rMap, logger)
	if len(rMap) == 0 {
		return
	}

	var engineResponses []*response.EngineResponse
	for _, resource := range rMap {
		responses := pc.applyPolicy(policy, resource, logger)
		engineResponses = append(engineResponses, responses...)
	}

	if !*metricAlreadyRegistered && len(engineResponses) > 0 {
		for _, engineResponse := range engineResponses {
			// registering the kyverno_policy_results_total metric concurrently
			go pc.registerPolicyResultsMetricValidation(logger, policy, *engineResponse)
			// registering the kyverno_policy_execution_duration_seconds metric concurrently
			go pc.registerPolicyExecutionDurationMetricValidate(logger, policy, *engineResponse)
		}
		*metricAlreadyRegistered = true
	}

	pc.report(engineResponses, logger)
}

func (pc *PolicyController) registerPolicyResultsMetricValidation(logger logr.Logger, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	if err := policyResults.ProcessEngineResponse(context.TODO(), pc.metricsConfig, policy, engineResponse, metrics.BackgroundScan, metrics.ResourceCreated); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_results_total metrics for the above policy", "name", policy.GetName())
	}
}

func (pc *PolicyController) registerPolicyExecutionDurationMetricValidate(logger logr.Logger, policy kyvernov1.PolicyInterface, engineResponse response.EngineResponse) {
	if err := policyExecutionDuration.ProcessEngineResponse(context.TODO(), pc.metricsConfig, policy, engineResponse, metrics.BackgroundScan, metrics.ResourceCreated); err != nil {
		logger.Error(err, "error occurred while registering kyverno_policy_execution_duration_seconds metrics for the above policy", "name", policy.GetName())
	}
}

func (pc *PolicyController) applyPolicy(policy kyvernov1.PolicyInterface, resource unstructured.Unstructured, logger logr.Logger) (engineResponses []*response.EngineResponse) {
	// pre-processing, check if the policy and resource version has been processed before
	if !pc.rm.ProcessResource(policy.GetName(), policy.GetResourceVersion(), resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion()) {
		logger.V(4).Info("policy and resource already processed", "policyResourceVersion", policy.GetResourceVersion(), "resourceResourceVersion", resource.GetResourceVersion(), "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	}

	namespaceLabels := engineutils.GetNamespaceSelectorsFromNamespaceLister(resource.GetKind(), resource.GetNamespace(), pc.nsLister, logger)
	engineResponse := applyPolicy(policy, resource, logger, pc.configHandler.GetExcludeGroupRole(), pc.client, pc.rclient, pc.informerCacheResolvers, namespaceLabels)
	engineResponses = append(engineResponses, engineResponse...)

	// post-processing, register the resource as processed
	pc.rm.RegisterResource(policy.GetName(), policy.GetResourceVersion(), resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion())

	return
}

func (pc *PolicyController) report(engineResponses []*response.EngineResponse, logger logr.Logger) {
	eventInfos := generateFailEvents(logger, engineResponses)
	pc.eventGen.Add(eventInfos...)

	if pc.configHandler.GetGenerateSuccessEvents() {
		successEventInfos := generateSuccessEvents(logger, engineResponses)
		pc.eventGen.Add(successEventInfos...)
	}
}

// forceReconciliation forces a background scan by adding all policies to the workqueue
func (pc *PolicyController) forceReconciliation(ctx context.Context) {
	logger := pc.log.WithName("forceReconciliation")
	ticker := time.NewTicker(pc.reconcilePeriod)

	for {
		select {
		case <-ticker.C:
			logger.Info("performing the background scan", "scan interval", pc.reconcilePeriod.String())
			pc.requeuePolicies()

		case <-ctx.Done():
			return
		}
	}
}

func (pc *PolicyController) requeuePolicies() {
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

func (pc *PolicyController) updateUR(policyKey string, policy kyvernov1.PolicyInterface) error {
	logger := pc.log.WithName("updateUR").WithName(policyKey)

	if !policy.GetSpec().MutateExistingOnPolicyUpdate && !policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
		logger.V(4).Info("skip policy application on policy event", "policyKey", policyKey, "mutateExiting", policy.GetSpec().MutateExistingOnPolicyUpdate, "generateExisting", policy.GetSpec().IsGenerateExistingOnPolicyUpdate())
		return nil
	}

	logger.Info("update URs on policy event")

	var errors []error
	mutateURs := pc.listMutateURs(policyKey, nil)
	generateURs := pc.listGenerateURs(policyKey, nil)
	updateUR(pc.kyvernoClient, pc.urLister.UpdateRequests(config.KyvernoNamespace()), policyKey, append(mutateURs, generateURs...), pc.log.WithName("updateUR"))

	for _, rule := range policy.GetSpec().Rules {
		var ruleType kyvernov1beta1.RequestType

		if rule.IsMutateExisting() {
			ruleType = kyvernov1beta1.Mutate

			triggers := generateTriggers(pc.client, rule, pc.log)
			for _, trigger := range triggers {
				murs := pc.listMutateURs(policyKey, trigger)

				if murs != nil {
					logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+trigger.GetName())
					continue
				}

				logger.Info("creating new UR for mutate")
				ur := newUR(policy, trigger, ruleType)
				skip, err := pc.handleUpdateRequest(ur, trigger, rule, policy)
				if err != nil {
					pc.log.Error(err, "failed to create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
					continue
				}
				if skip {
					continue
				}
				pc.log.V(2).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}
		}

		if policy.GetSpec().IsGenerateExistingOnPolicyUpdate() {
			ruleType = kyvernov1beta1.Generate
			triggers := generateTriggers(pc.client, rule, pc.log)
			for _, trigger := range triggers {
				gurs := pc.listGenerateURs(policyKey, trigger)

				if gurs != nil {
					logger.V(4).Info("UR was created", "rule", rule.Name, "rule type", ruleType, "trigger", trigger.GetNamespace()+"/"+trigger.GetName())
					continue
				}

				ur := newUR(policy, trigger, ruleType)
				skip, err := pc.handleUpdateRequest(ur, trigger, rule, policy)
				if err != nil {
					pc.log.Error(err, "failed to create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
					errors = append(errors, err)
					continue
				}

				if skip {
					continue
				}

				pc.log.V(4).Info("successfully created UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule type", ruleType,
					"target", fmt.Sprintf("%s/%s/%s/%s", trigger.GetAPIVersion(), trigger.GetKind(), trigger.GetNamespace(), trigger.GetName()))
			}

			err := multierr.Combine(errors...)
			return err
		}
	}

	return nil
}

func (pc *PolicyController) handleUpdateRequest(ur *kyvernov1beta1.UpdateRequest, triggerResource *unstructured.Unstructured, rule kyvernov1.Rule, policy kyvernov1.PolicyInterface) (skip bool, err error) {
	policyContext, _, err := backgroundcommon.NewBackgroundContext(pc.client, ur, policy, triggerResource, pc.configHandler, pc.informerCacheResolvers, nil, pc.log)
	if err != nil {
		return false, errors.Wrapf(err, "failed to build policy context for rule %s", rule.Name)
	}

	engineResponse := engine.ApplyBackgroundChecks(pc.rclient, policyContext)
	if len(engineResponse.PolicyResponse.Rules) == 0 {
		return true, nil
	}

	for _, ruleResponse := range engineResponse.PolicyResponse.Rules {
		if ruleResponse.Status != response.RuleStatusPass {
			pc.log.Error(err, "can not create new UR on policy update", "policy", policy.GetName(), "rule", rule.Name, "rule.Status", ruleResponse.Status)
			continue
		}

		pc.log.V(2).Info("creating new UR for generate")
		_, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace()).Create(context.TODO(), ur, metav1.CreateOptions{})
		if err != nil {
			return false, err
		}
	}
	return false, err
}

func (pc *PolicyController) listMutateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	mutateURs, err := pc.urLister.List(labels.SelectorFromSet(backgroundcommon.MutateLabelsSet(policyKey, trigger)))
	if err != nil {
		pc.log.Error(err, "failed to list update request for mutate policy")
	}
	return mutateURs
}

func (pc *PolicyController) listGenerateURs(policyKey string, trigger *unstructured.Unstructured) []*kyvernov1beta1.UpdateRequest {
	generateURs, err := pc.urLister.List(labels.SelectorFromSet(backgroundcommon.GenerateLabelsSet(policyKey, trigger)))
	if err != nil {
		pc.log.Error(err, "failed to list update request for generate policy")
	}
	return generateURs
}

func (pc *PolicyController) getResourceList(kind, namespace string, labelSelector *metav1.LabelSelector, log logr.Logger) *unstructured.UnstructuredList {
	gv, k := kubeutils.GetKindFromGVK(kind)
	resourceList, err := pc.client.ListResource(context.TODO(), gv, k, namespace, labelSelector)
	if err != nil {
		log.Error(err, "failed to list resources", "kind", k, "namespace", namespace)
		return nil
	}
	return resourceList
}

// GetResourcesPerNamespace returns
// - Namespaced resources across all namespaces if namespace is set to empty "", for Namespaced Kind
// - Namespaced resources in the given namespace
// - Cluster-wide resources for Cluster-wide Kind
func (pc *PolicyController) getResourcesPerNamespace(kind string, namespace string, rule kyvernov1.Rule, log logr.Logger) map[string]unstructured.Unstructured {
	resourceMap := map[string]unstructured.Unstructured{}

	if kind == "Namespace" {
		namespace = ""
	}

	list := pc.getResourceList(kind, namespace, rule.MatchResources.Selector, log)
	if list != nil {
		for _, r := range list.Items {
			if pc.match(r, rule) {
				resourceMap[string(r.GetUID())] = r
			}
		}
	}

	// skip resources to be filtered
	excludeResources(resourceMap, rule.ExcludeResources.ResourceDescription, pc.configHandler, log)
	return resourceMap
}

func (pc *PolicyController) match(r unstructured.Unstructured, rule kyvernov1.Rule) bool {
	if r.GetDeletionTimestamp() != nil {
		return false
	}

	if r.GetKind() == "Pod" {
		if !isRunningPod(r) {
			return false
		}
	}

	// match name
	if rule.MatchResources.Name != "" {
		if !wildcard.Match(rule.MatchResources.Name, r.GetName()) {
			return false
		}
	}
	// Skip the filtered resources
	if pc.configHandler.ToFilter(r.GetKind(), r.GetNamespace(), r.GetName()) {
		return false
	}

	return true
}

func (pc *PolicyController) processExistingKinds(kinds []string, policy kyvernov1.PolicyInterface, rule kyvernov1.Rule, logger logr.Logger) {
	for _, kind := range kinds {
		logger = logger.WithValues("rule", rule.Name, "kind", kind)
		_, err := pc.rm.GetScope(kind)
		if err != nil {
			gv, k := kubeutils.GetKindFromGVK(kind)
			if !strings.Contains(k, "*") {
				resourceSchema, _, _, err := pc.client.Discovery().FindResource(gv, k)
				if err != nil {
					logger.Error(err, "failed to find resource", "kind", k)
					continue
				}
				pc.rm.RegisterScope(k, resourceSchema.Namespaced)
			}
		}

		// this tracker would help to ensure that even for multiple namespaces, duplicate metric are not generated
		metricRegisteredTracker := false

		if policy.GetNamespace() != "" {
			ns := policy.GetNamespace()
			pc.applyAndReportPerNamespace(policy, kind, ns, rule, logger.WithValues("kind", kind).WithValues("ns", ns), &metricRegisteredTracker)
			continue
		}

		pc.applyAndReportPerNamespace(policy, kind, "", rule, logger.WithValues("kind", kind), &metricRegisteredTracker)
	}
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

func updateUR(kyvernoClient versioned.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister, policyKey string, urList []*kyvernov1beta1.UpdateRequest, logger logr.Logger) {
	for _, ur := range urList {
		if policyKey == ur.Spec.Policy {
			_, err := backgroundcommon.Update(kyvernoClient, urLister, ur.GetName(), func(ur *kyvernov1beta1.UpdateRequest) {
				urLabels := ur.Labels
				if len(urLabels) == 0 {
					urLabels = make(map[string]string)
				}
				nBig, err := rand.Int(rand.Reader, big.NewInt(100000))
				if err != nil {
					logger.Error(err, "failed to generate random interger")
				}
				urLabels["policy-update"] = fmt.Sprintf("revision-count-%d", nBig.Int64())
				ur.SetLabels(urLabels)
			})
			if err != nil {
				logger.Error(err, "failed to update gr", "name", ur.GetName())
				continue
			}
			if _, err := backgroundcommon.UpdateStatus(kyvernoClient, urLister, ur.GetName(), kyvernov1beta1.Pending, "", nil); err != nil {
				logger.Error(err, "failed to set UpdateRequest state to Pending")
			}
		}
	}
}

func missingAutoGenRules(policy kyvernov1.PolicyInterface, log logr.Logger) bool {
	var podRuleName []string
	ruleCount := 1
	spec := policy.GetSpec()
	if canApplyAutoGen, _ := autogen.CanAutoGen(spec); canApplyAutoGen {
		for _, rule := range autogen.ComputeRules(policy) {
			podRuleName = append(podRuleName, rule.Name)
		}
	}

	if len(podRuleName) > 0 {
		annotations := policy.GetAnnotations()
		val, ok := annotations[kyvernov1.PodControllersAnnotation]
		if !ok {
			return true
		}
		if val == "none" {
			return false
		}
		res := strings.Split(val, ",")

		if len(res) == 1 {
			ruleCount = 2
		}
		if len(res) > 1 {
			if slices.Contains(res, "CronJob") {
				ruleCount = 3
			} else {
				ruleCount = 2
			}
		}

		if len(autogen.ComputeRules(policy)) != (ruleCount * len(podRuleName)) {
			return true
		}
	}
	return false
}
