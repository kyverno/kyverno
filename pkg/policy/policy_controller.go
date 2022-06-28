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
	utilscommon "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/autogen"
	common "github.com/kyverno/kyverno/pkg/background/common"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	kyvernov1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
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
	client        client.Interface
	kyvernoClient kyvernoclient.Interface
	pInformer     kyvernoinformer.ClusterPolicyInformer
	npInformer    kyvernoinformer.PolicyInformer

	eventGen      event.Interface
	eventRecorder record.EventRecorder

	// Policies that need to be synced
	queue workqueue.RateLimitingInterface

	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister

	// urLister can list/get update request from the shared informer's store
	urLister urkyvernolister.UpdateRequestLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister listerv1.NamespaceLister

	informersSynced []cache.InformerSynced

	// Resource manager, manages the mapping for already processed resource
	rm resourceManager

	// helpers to validate against current loaded configuration
	configHandler config.Configuration

	// policy report generator
	prGenerator policyreport.GeneratorInterface

	policyReportEraser policyreport.PolicyReportEraser

	reconcilePeriod time.Duration

	log logr.Logger

	promConfig *metrics.PromConfig
}

// NewPolicyController create a new PolicyController
func NewPolicyController(
	kubeClient kubernetes.Interface,
	kyvernoClient kyvernoclient.Interface,
	client client.Interface,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	urInformer urkyvernoinformer.UpdateRequestInformer,
	configHandler config.Configuration,
	eventGen event.Interface,
	prGenerator policyreport.GeneratorInterface,
	policyReportEraser policyreport.PolicyReportEraser,
	namespaces informers.NamespaceInformer,
	log logr.Logger,
	reconcilePeriod time.Duration,
	promConfig *metrics.PromConfig,
) (*PolicyController, error) {

	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.V(5).Info)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		return nil, err
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pc := PolicyController{
		client:             client,
		kyvernoClient:      kyvernoClient,
		pInformer:          pInformer,
		npInformer:         npInformer,
		eventGen:           eventGen,
		eventRecorder:      eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "policy_controller"}),
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		configHandler:      configHandler,
		prGenerator:        prGenerator,
		policyReportEraser: policyReportEraser,
		reconcilePeriod:    reconcilePeriod,
		promConfig:         promConfig,
		log:                log,
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
		logger.V(4).Info("background processed is disabled")
		return false
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

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricAddPolicy(logger, p)

	if !toggle.AutogenInternals() {
		if p.Spec.Background == nil || p.Spec.ValidationFailureAction == "" || missingAutoGenRules(p, logger) {
			pol, _ := utilscommon.MutatePolicy(p, logger)
			_, err := pc.kyvernoClient.KyvernoV1().ClusterPolicies().Update(context.TODO(), pol.(*kyvernov1.ClusterPolicy), metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to add policy ")
			}
		}
	}

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

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)

	if !toggle.AutogenInternals() {
		if curP.Spec.Background == nil || curP.Spec.ValidationFailureAction == "" || missingAutoGenRules(curP, logger) {
			pol, _ := utilscommon.MutatePolicy(curP, logger)
			_, err := pc.kyvernoClient.KyvernoV1().ClusterPolicies().Update(context.TODO(), pol.(*kyvernov1.ClusterPolicy), metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to update policy ")
			}
		}
	}

	if !pc.canBackgroundProcess(curP) {
		return
	}

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(2).Info("updating policy", "name", oldP.Name)

	pc.enqueueRCRDeletedRule(oldP, curP)
	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deletePolicy(obj interface{}) {
	logger := pc.log
	p, ok := obj.(*kyvernov1.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tombstone", "obj", obj)
			return
		}
		p, ok = tombstone.Obj.(*kyvernov1.ClusterPolicy)
		if !ok {
			logger.Info("tombstone container object that is not a policy", "obj", obj)
			return
		}
	}

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricDeletePolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricDeletePolicy(logger, p)

	logger.Info("policy deleted", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	pc.enqueueRCRDeletedPolicy(p.Name)

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

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricAddPolicy(logger, p)

	logger.Info("policy created", "uid", p.UID, "kind", "Policy", "name", p.Name, "namespaces", p.Namespace)

	if !toggle.AutogenInternals() {
		spec := p.GetSpec()
		if spec.Background == nil || spec.ValidationFailureAction == "" || missingAutoGenRules(p, logger) {
			nsPol, _ := utilscommon.MutatePolicy(p, logger)
			_, err := pc.kyvernoClient.KyvernoV1().Policies(p.Namespace).Update(context.TODO(), nsPol.(*kyvernov1.Policy), metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to add namespace policy")
			}
		}
	}

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

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)

	if !toggle.AutogenInternals() {
		if curP.Spec.Background == nil || curP.Spec.ValidationFailureAction == "" || missingAutoGenRules(curP, logger) {
			nsPol, _ := utilscommon.MutatePolicy(curP, logger)
			_, err := pc.kyvernoClient.KyvernoV1().Policies(curP.GetNamespace()).Update(context.TODO(), nsPol.(*kyvernov1.Policy), metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to update namespace policy ")
			}
		}
	}

	if !pc.canBackgroundProcess(curP) {
		return
	}

	if reflect.DeepEqual(oldP.Spec, curP.Spec) {
		return
	}

	logger.V(4).Info("updating namespace policy", "namespace", oldP.Namespace, "name", oldP.Name)

	pc.enqueueRCRDeletedRule(oldP, curP)
	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deleteNsPolicy(obj interface{}) {
	logger := pc.log
	p, ok := obj.(*kyvernov1.Policy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tombstone", "obj", obj)
			return
		}
		p, ok = tombstone.Obj.(*kyvernov1.Policy)
		if !ok {
			logger.Info("tombstone container object that is not a policy", "obj", obj)
			return
		}
	}

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricDeletePolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricDeletePolicy(logger, p)

	logger.Info("policy deleted event", "uid", p.UID, "kind", "Policy", "policy_name", p.Name, "namespaces", p.Namespace)

	pol := p

	pc.enqueueRCRDeletedPolicy(p.Name)

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

func (pc *PolicyController) enqueueRCRDeletedRule(old, cur kyvernov1.PolicyInterface) {
	curRule := make(map[string]bool)
	for _, rule := range autogen.ComputeRules(cur) {
		curRule[rule.Name] = true
	}

	for _, rule := range autogen.ComputeRules(old) {
		if !curRule[rule.Name] {
			pc.prGenerator.Add(policyreport.Info{
				PolicyName: cur.GetName(),
				Results: []policyreport.EngineResponseResult{
					{
						Rules: []kyvernov1.ViolatedRule{
							{Name: rule.Name},
						},
					},
				},
			})
		}
	}
}

func (pc *PolicyController) enqueueRCRDeletedPolicy(policyName string) {
	pc.prGenerator.Add(policyreport.Info{
		PolicyName: policyName,
	})
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
func (pc *PolicyController) Run(workers int, reconcileCh <-chan bool, cleanupChangeRequest <-chan policyreport.ReconcileInfo, stopCh <-chan struct{}) {
	logger := pc.log

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForNamedCacheSync("PolicyController", stopCh, pc.informersSynced...) {
		return
	}

	pc.pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	pc.npInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})

	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, time.Second, stopCh)
	}

	go pc.forceReconciliation(reconcileCh, cleanupChangeRequest, stopCh)

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pc *PolicyController) worker() {
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
		if errors.IsNotFound(err) {
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
	namespace, key, isNamespacedPolicy := ParseNamespacedPolicy(key)
	if !isNamespacedPolicy {
		return pc.pLister.Get(key)
	}
	return pc.npLister.Policies(namespace).Get(key)
}

func generateTriggers(client client.Interface, rule kyvernov1.Rule, log logr.Logger) []*unstructured.Unstructured {
	list := &unstructured.UnstructuredList{}

	kinds := fetchUniqueKinds(rule)

	for _, kind := range kinds {
		mlist, err := client.ListResource("", kind, "", rule.MatchResources.Selector)
		if err != nil {
			log.Error(err, "failed to list matched resource")
		}
		list.Items = append(list.Items, mlist.Items...)
	}
	return convertlist(list.Items)
}

func updateUR(kyvernoClient kyvernoclient.Interface, urLister kyvernov1beta1listers.UpdateRequestNamespaceLister, policyKey string, urList []*kyvernov1beta1.UpdateRequest, logger logr.Logger) {
	for _, ur := range urList {
		if policyKey == ur.Spec.Policy {
			_, err := common.Update(kyvernoClient, urLister, ur.GetName(), func(ur *kyvernov1beta1.UpdateRequest) {
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
			if _, err := common.UpdateStatus(kyvernoClient, urLister, ur.GetName(), kyvernov1beta1.Pending, "", nil); err != nil {
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
			if utils.ContainsString(res, "CronJob") {
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
