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
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	urkyverno "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	"github.com/kyverno/kyverno/pkg/autogen"
	kyvernoclient "github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	urkyvernoinformer "github.com/kyverno/kyverno/pkg/client/informers/externalversions/kyverno/v1beta1"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	urkyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1beta1"
	pkgCommon "github.com/kyverno/kyverno/pkg/common"
	"github.com/kyverno/kyverno/pkg/config"
	client "github.com/kyverno/kyverno/pkg/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/kyverno/kyverno/pkg/metrics"
	"github.com/kyverno/kyverno/pkg/policyreport"
	"github.com/kyverno/kyverno/pkg/utils"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset
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

	// grLister can list/get generate request from the shared informer's store
	grLister kyvernolister.GenerateRequestLister

	// urLister can list/get update request from the shared informer's store
	urLister urkyvernolister.UpdateRequestLister

	// nsLister can list/get namespaces from the shared informer's store
	nsLister listerv1.NamespaceLister

	// pListerSynced returns true if the cluster policy store has been synced at least once
	pListerSynced cache.InformerSynced

	// npListerSynced returns true if the namespace policy store has been synced at least once
	npListerSynced cache.InformerSynced

	// nsListerSynced returns true if the namespace store has been synced at least once
	nsListerSynced cache.InformerSynced

	// grListerSynced returns true if the generate request store has been synced at least once
	grListerSynced cache.InformerSynced

	// urListerSynced returns true if the update request store has been synced at least once
	urListerSynced cache.InformerSynced

	// Resource manager, manages the mapping for already processed resource
	rm resourceManager

	// helpers to validate against current loaded configuration
	configHandler config.Interface

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
	kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	grInformer kyvernoinformer.GenerateRequestInformer,
	urInformer urkyvernoinformer.UpdateRequestInformer,
	configHandler config.Interface,
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
	pc.grLister = grInformer.Lister()
	pc.urLister = urInformer.Lister()

	pc.pListerSynced = pInformer.Informer().HasSynced
	pc.npListerSynced = npInformer.Informer().HasSynced

	pc.nsListerSynced = namespaces.Informer().HasSynced
	pc.grListerSynced = grInformer.Informer().HasSynced
	pc.urListerSynced = urInformer.Informer().HasSynced

	// resource manager
	// rebuild after 300 seconds/ 5 mins
	pc.rm = NewResourceManager(30)

	return &pc, nil
}

func (pc *PolicyController) canBackgroundProcess(p kyverno.PolicyInterface) bool {
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
	p := obj.(*kyverno.ClusterPolicy)

	logger.Info("policy created", "uid", p.UID, "kind", "ClusterPolicy", "name", p.Name)

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricAddPolicy(logger, p)

	if p.Spec.Background == nil || p.Spec.ValidationFailureAction == "" || missingAutoGenRules(p, logger) {
		pol, _ := common.MutatePolicy(p, logger)
		_, err := pc.client.UpdateResource("kyverno.io/v1", "ClusterPolicy", "", pol, false)
		if err != nil {
			logger.Error(err, "failed to add policy ")
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
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)

	if curP.Spec.Background == nil || curP.Spec.ValidationFailureAction == "" || missingAutoGenRules(curP, logger) {
		pol, _ := common.MutatePolicy(curP, logger)
		_, err := pc.client.UpdateResource("kyverno.io/v1", "ClusterPolicy", "", pol, false)
		if err != nil {
			logger.Error(err, "failed to update policy ")
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
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tombstone", "obj", obj)
			return
		}

		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
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

	// we process policies that are not set of background processing
	// as we need to clean up GRs when a policy is deleted
	// skip generate policies with clone
	rules := autogen.ComputeRules(p)

	generatePolicyWithClone := pkgCommon.ProcessDeletePolicyForCloneGenerateRule(rules, pc.client, p.GetName(), logger)

	if !generatePolicyWithClone {
		pc.enqueuePolicy(p)
		pc.enqueueRCRDeletedPolicy(p.Name)
	}
}

func (pc *PolicyController) addNsPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyverno.Policy)

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricAddPolicy(logger, p)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricAddPolicy(logger, p)

	logger.Info("policy created", "uid", p.UID, "kind", "Policy", "name", p.Name, "namespaces", p.Namespace)

	spec := p.GetSpec()
	if spec.Background == nil || spec.ValidationFailureAction == "" || missingAutoGenRules(p, logger) {
		nsPol, _ := common.MutatePolicy(p, logger)
		_, err := pc.client.UpdateResource("kyverno.io/v1", "Policy", p.Namespace, nsPol, false)
		if err != nil {
			logger.Error(err, "failed to add namespace policy")
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
	oldP := old.(*kyverno.Policy)
	curP := cur.(*kyverno.Policy)

	// register kyverno_policy_rule_info_total metric concurrently
	go pc.registerPolicyRuleInfoMetricUpdatePolicy(logger, oldP, curP)
	// register kyverno_policy_changes_total metric concurrently
	go pc.registerPolicyChangesMetricUpdatePolicy(logger, oldP, curP)

	if curP.Spec.Background == nil || curP.Spec.ValidationFailureAction == "" || missingAutoGenRules(curP, logger) {
		nsPol, _ := common.MutatePolicy(curP, logger)
		_, err := pc.client.UpdateResource("kyverno.io/v1", "Policy", curP.GetNamespace(), nsPol, false)
		if err != nil {
			logger.Error(err, "failed to update namespace policy ")
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
	p, ok := obj.(*kyverno.Policy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tombstone", "obj", obj)
			return
		}

		p, ok = tombstone.Obj.(*kyverno.Policy)
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

	// we process policies that are not set of background processing
	// as we need to clean up GRs when a policy is deleted
	pc.enqueuePolicy(pol)
	pc.enqueueRCRDeletedPolicy(p.Name)
}

func (pc *PolicyController) enqueueRCRDeletedRule(old, cur kyverno.PolicyInterface) {
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
						Rules: []kyverno.ViolatedRule{
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

func (pc *PolicyController) enqueuePolicy(policy kyverno.PolicyInterface) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *PolicyController) Run(workers int, reconcileCh <-chan bool, stopCh <-chan struct{}) {
	logger := pc.log

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.npListerSynced, pc.nsListerSynced, pc.grListerSynced, pc.urListerSynced) {
		logger.Info("failed to sync informer cache")
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

	go pc.forceReconciliation(reconcileCh, stopCh)

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

	urList, err := pc.urLister.List(labels.Everything())
	if err != nil {
		logger.Error(err, "failed to list update request")
	}

	policy, err := pc.getPolicy(key)
	if err != nil {
		if errors.IsNotFound(err) {
			deleteGR(pc.kyvernoClient, key, urList, logger)
			return nil
		}

		return err
	}

	pc.updateUR(policy, urList)
	pc.processExistingResources(policy)
	return nil
}

func (pc *PolicyController) getPolicy(key string) (policy kyverno.PolicyInterface, err error) {
	namespace, key, isNamespacedPolicy := ParseNamespacedPolicy(key)
	if !isNamespacedPolicy {
		return pc.pLister.Get(key)
	}

	nsPolicy, err := pc.npLister.Policies(namespace).Get(key)
	if err == nil && nsPolicy != nil {
		policy = nsPolicy
	}

	return
}

func (pc *PolicyController) updateUR(policy kyverno.PolicyInterface, urList []*urkyverno.UpdateRequest) {
	if urList == nil {
		for _, rule := range policy.GetSpec().Rules {
			if !rule.IsMutateExisting() {
				continue
			}

			triggers := getTriggers(rule)
			for _, trigger := range triggers {
				ur := newUR(policy, trigger)
				new, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Create(context.TODO(), ur, metav1.CreateOptions{})
				if err != nil {
					pc.log.Error(err, "failed to create new UR for mutateExisting rule on policy update", "policy", policy.GetName(), "rule", rule.Name,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
				} else {
					pc.log.V(4).Info("successfully created UR for mutateExisting on policy update", "policy", policy.GetName(), "rule", rule.Name,
						"target", fmt.Sprintf("%s/%s/%s/%s", trigger.APIVersion, trigger.Kind, trigger.Namespace, trigger.Name))
				}

				new.Status.State = urkyverno.Pending
				if _, err := pc.kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).UpdateStatus(context.TODO(), new, metav1.UpdateOptions{}); err != nil {
					pc.log.Error(err, "failed to set UpdateRequest state to Pending")
				}
			}
		}
		return
	}

	updateUR(pc.kyvernoClient, policy.GetName(), urList, pc.log.WithName("updateUR"))

}

func getTriggers(rule kyverno.Rule) []*kyverno.ResourceSpec {
	var specs []*kyverno.ResourceSpec

	triggers := getTrigger(rule.MatchResources.ResourceDescription)
	specs = append(specs, triggers...)

	for _, any := range rule.MatchResources.Any {
		triggers := getTrigger(any.ResourceDescription)
		specs = append(specs, triggers...)
	}

	triggers = []*kyverno.ResourceSpec{}
	for _, all := range rule.MatchResources.All {
		triggers = getTrigger(all.ResourceDescription)
	}

	subset := make(map[*kyverno.ResourceSpec]int, len(triggers))
	for _, trigger := range triggers {
		c := subset[trigger]
		subset[trigger] = c + 1
	}

	for k, v := range subset {
		if v == len(rule.MatchResources.All) {
			specs = append(specs, k)
		}
	}
	return specs
}

func getTrigger(rd kyverno.ResourceDescription) []*kyverno.ResourceSpec {
	if len(rd.Names) == 0 {
		return nil
	}

	var specs []*kyverno.ResourceSpec

	for _, k := range rd.Kinds {
		apiVersion, kind := kubeutils.GetKindFromGVK(k)
		if kind == "" {
			continue
		}

		for _, ns := range rd.Namespaces {
			for _, name := range rd.Names {
				if name == "" {
					continue
				}

				spec := &kyverno.ResourceSpec{
					APIVersion: apiVersion,
					Kind:       kind,
					Namespace:  ns,
					Name:       name,
				}
				specs = append(specs, spec)
			}
		}
	}

	return specs
}

func deleteGR(kyvernoClient *kyvernoclient.Clientset, policyKey string, grList []*urkyverno.UpdateRequest, logger logr.Logger) {
	for _, v := range grList {
		if policyKey == v.Spec.Policy {
			err := kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Delete(context.TODO(), v.GetName(), metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				logger.Error(err, "failed to delete ur", "name", v.GetName())
			}
		}
	}
}

func updateUR(kyvernoClient *kyvernoclient.Clientset, policyKey string, grList []*urkyverno.UpdateRequest, logger logr.Logger) {
	for _, gr := range grList {
		if policyKey == gr.Spec.Policy {
			grLabels := gr.Labels
			if len(grLabels) == 0 {
				grLabels = make(map[string]string)
			}

			nBig, err := rand.Int(rand.Reader, big.NewInt(100000))
			if err != nil {
				logger.Error(err, "failed to generate random interger")
			}
			grLabels["policy-update"] = fmt.Sprintf("revision-count-%d", nBig.Int64())
			gr.SetLabels(grLabels)

			_, err = kyvernoClient.KyvernoV1beta1().UpdateRequests(config.KyvernoNamespace).Update(context.TODO(), gr, metav1.UpdateOptions{})
			if err != nil {
				logger.Error(err, "failed to update gr", "name", gr.GetName())
			}
		}
	}
}

func missingAutoGenRules(policy kyverno.PolicyInterface, log logr.Logger) bool {
	var podRuleName []string
	ruleCount := 1
	spec := policy.GetSpec()
	if canApplyAutoGen, _ := autogen.CanAutoGen(spec, log); canApplyAutoGen {
		for _, rule := range autogen.ComputeRules(policy) {
			podRuleName = append(podRuleName, rule.Name)
		}
	}

	if len(podRuleName) > 0 {
		annotations := policy.GetAnnotations()
		val, ok := annotations[kyverno.PodControllersAnnotation]
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

func newUR(policy kyverno.PolicyInterface, target *kyverno.ResourceSpec) *urkyverno.UpdateRequest {
	var policyNameNamespaceKey string

	if policy.GetKind() == "Policy" {
		policyNameNamespaceKey = policy.GetNamespace() + "/" + policy.GetName()
	} else {
		policyNameNamespaceKey = policy.GetName()
	}

	label := map[string]string{
		"mutate.updaterequest.kyverno.io/policy-name":       policyNameNamespaceKey,
		"mutate.updaterequest.kyverno.io/trigger-name":      target.Name,
		"mutate.updaterequest.kyverno.io/trigger-namespace": target.Namespace,
		"mutate.updaterequest.kyverno.io/trigger-kind":      target.Kind,
	}

	if target.APIVersion != "" {
		label["mutate.updaterequest.kyverno.io/trigger-apiversion"] = target.APIVersion
	}

	return &urkyverno.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ur-",
			Namespace:    config.KyvernoNamespace,
			Labels:       label,
		},
		Spec: urkyverno.UpdateRequestSpec{
			Type:   urkyverno.Mutate,
			Policy: policyNameNamespaceKey,
			Resource: kyverno.ResourceSpec{
				Kind:       target.Kind,
				Namespace:  target.Namespace,
				Name:       target.Name,
				APIVersion: target.APIVersion,
			},
		},
	}
}
