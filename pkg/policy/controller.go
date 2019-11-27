package policy

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policystore"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	mconfiginformer "k8s.io/client-go/informers/admissionregistration/v1beta1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	mconfiglister "k8s.io/client-go/listers/admissionregistration/v1beta1"
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

var controllerKind = kyverno.SchemeGroupVersion.WithKind("ClusterPolicy")

// PolicyController is responsible for synchronizing Policy objects stored
// in the system with the corresponding policy violations
type PolicyController struct {
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset
	eventGen      event.Interface
	eventRecorder record.EventRecorder
	syncHandler   func(pKey string) error
	enqueuePolicy func(policy *kyverno.ClusterPolicy)

	//pvControl is used for adoptin/releasing policy violation
	pvControl PVControlInterface
	// Policys that need to be synced
	queue workqueue.RateLimitingInterface
	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// pvLister can list/get policy violation from the shared informer's store
	cpvLister kyvernolister.ClusterPolicyViolationLister
	// nspvLister can list/get namespaced policy violation from the shared informer's store
	nspvLister kyvernolister.NamespacedPolicyViolationLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// pvListerSynced returns true if the Policy store has been synced at least once
	cpvListerSynced cache.InformerSynced
	// pvListerSynced returns true if the Policy Violation store has been synced at least once
	nspvListerSynced cache.InformerSynced
	// mwebhookconfigSynced returns true if the Mutating Webhook Config store has been synced at least once
	mwebhookconfigSynced cache.InformerSynced
	// list/get mutatingwebhookconfigurations
	mWebhookConfigLister mconfiglister.MutatingWebhookConfigurationLister
	// WebhookRegistrationClient
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient
	// Resource manager, manages the mapping for already processed resource
	rm resourceManager
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// recieves stats and aggregates details
	statusAggregator *PolicyStatusAggregator
	// store to hold policy meta data for faster lookup
	pMetaStore policystore.UpdateInterface
	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface
}

// NewPolicyController create a new PolicyController
func NewPolicyController(kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	cpvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	nspvInformer kyvernoinformer.NamespacedPolicyViolationInformer,
	mconfigwebhookinformer mconfiginformer.MutatingWebhookConfigurationInformer,
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient,
	configHandler config.Interface,
	eventGen event.Interface,
	pvGenerator policyviolation.GeneratorInterface,
	pMetaStore policystore.UpdateInterface) (*PolicyController, error) {
	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		return nil, err
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pc := PolicyController{
		client:                    client,
		kyvernoClient:             kyvernoClient,
		eventGen:                  eventGen,
		eventRecorder:             eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "policy_controller"}),
		queue:                     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		webhookRegistrationClient: webhookRegistrationClient,
		configHandler:             configHandler,
		pMetaStore:                pMetaStore,
		pvGenerator:               pvGenerator,
	}

	pc.pvControl = RealPVControl{Client: kyvernoClient, Recorder: pc.eventRecorder}

	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	cpvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addClusterPolicyViolation,
		UpdateFunc: pc.updateClusterPolicyViolation,
		DeleteFunc: pc.deleteClusterPolicyViolation,
	})

	nspvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNamespacedPolicyViolation,
		UpdateFunc: pc.updateNamespacedPolicyViolation,
		DeleteFunc: pc.deleteNamespacedPolicyViolation,
	})

	pc.enqueuePolicy = pc.enqueue
	pc.syncHandler = pc.syncPolicy

	pc.pLister = pInformer.Lister()
	pc.cpvLister = cpvInformer.Lister()
	pc.nspvLister = nspvInformer.Lister()

	pc.pListerSynced = pInformer.Informer().HasSynced
	pc.cpvListerSynced = cpvInformer.Informer().HasSynced
	pc.nspvListerSynced = nspvInformer.Informer().HasSynced
	pc.mwebhookconfigSynced = mconfigwebhookinformer.Informer().HasSynced
	pc.mWebhookConfigLister = mconfigwebhookinformer.Lister()
	// resource manager
	// rebuild after 300 seconds/ 5 mins
	//TODO: pass the time in seconds instead of converting it internally
	pc.rm = NewResourceManager(30)

	// aggregator
	// pc.statusAggregator = NewPolicyStatAggregator(kyvernoClient, pInformer)
	pc.statusAggregator = NewPolicyStatAggregator(kyvernoClient)

	return &pc, nil
}

func (pc *PolicyController) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	glog.V(4).Infof("Adding Policy %s", p.Name)
	// register with policy meta-store
	pc.pMetaStore.Register(*p)
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) updatePolicy(old, cur interface{}) {
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)
	glog.V(4).Infof("Updating Policy %s", oldP.Name)
	// TODO: optimize this : policy meta-store
	// Update policy-> (remove,add)
	pc.pMetaStore.UnRegister(*oldP)
	pc.pMetaStore.Register(*curP)
	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deletePolicy(obj interface{}) {
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a Policy %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting Policy %s", p.Name)
	// Unregister from policy meta-store
	pc.pMetaStore.UnRegister(*p)
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) enqueue(policy *kyverno.ClusterPolicy) {
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		glog.Error(err)
		return
	}
	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *PolicyController) Run(workers int, stopCh <-chan struct{}) {

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	glog.Info("Starting policy controller")
	defer glog.Info("Shutting down policy controller")

	if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.cpvListerSynced, pc.nspvListerSynced, pc.mwebhookconfigSynced) {
		glog.Error("failed to sync informer cache")
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, time.Second, stopCh)
	}
	// policy status aggregator
	//TODO: workers required for aggergation
	pc.statusAggregator.Run(1, stopCh)
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
	err := pc.syncHandler(key.(string))
	pc.handleErr(err, key)

	return true
}

func (pc *PolicyController) handleErr(err error, key interface{}) {
	if err == nil {
		pc.queue.Forget(key)
		return
	}

	if pc.queue.NumRequeues(key) < maxRetries {
		glog.V(2).Infof("Error syncing Policy %v: %v", key, err)
		pc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(2).Infof("Dropping policy %q out of the queue: %v", key, err)
	pc.queue.Forget(key)
}

func (pc *PolicyController) syncPolicy(key string) error {
	startTime := time.Now()
	glog.V(4).Infof("Started syncing policy %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing policy %q (%v)", key, time.Since(startTime))
	}()
	policy, err := pc.pLister.Get(key)
	if errors.IsNotFound(err) {
		glog.V(2).Infof("Policy %v has been deleted", key)
		// delete cluster policy violation
		if err := pc.deleteClusterPolicyViolations(policy); err != nil {
			return err
		}
		// delete namespaced policy violation
		if err := pc.deleteNamespacedPolicyViolations(policy); err != nil {
			return err
		}
		// remove the recorded stats for the policy
		pc.statusAggregator.RemovePolicyStats(key)

		// remove webhook configurations if there are no policies
		if err := pc.removeResourceWebhookConfiguration(); err != nil {
			// do not fail, if unable to delete resource webhook config
			glog.V(4).Infof("failed to remove resource webhook configuration: %v", err)
			glog.Errorln(err)
		}
		return nil
	}
	if err != nil {
		return err
	}

	if err := pc.createResourceMutatingWebhookConfigurationIfRequired(*policy); err != nil {
		glog.V(4).Infof("failed to create resource mutating webhook configurations, policies wont be applied on resources: %v", err)
		glog.Errorln(err)
	}

	// cluster policy violations
	cpvList, err := pc.getClusterPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}
	// namespaced policy violation
	nspvList, err := pc.getNamespacedPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}

	// process policies on existing resources
	engineResponses := pc.processExistingResources(*policy)
	// report errors
	pc.cleanupAndReport(engineResponses)
	// sync active
	return pc.syncStatusOnly(policy, cpvList, nspvList)
}

func (pc *PolicyController) deleteClusterPolicyViolations(policy *kyverno.ClusterPolicy) error {
	cpvList, err := pc.getClusterPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}
	for _, cpv := range cpvList {
		if err := pc.pvControl.DeleteClusterPolicyViolation(cpv.Name); err != nil {
			return err
		}
	}
	return nil
}

func (pc *PolicyController) deleteNamespacedPolicyViolations(policy *kyverno.ClusterPolicy) error {
	nspvList, err := pc.getNamespacedPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}
	for _, nspv := range nspvList {
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(nspv.Namespace, nspv.Name); err != nil {
			return err
		}
	}
	return nil
}

//syncStatusOnly updates the policy status subresource
func (pc *PolicyController) syncStatusOnly(p *kyverno.ClusterPolicy, pvList []*kyverno.ClusterPolicyViolation, nspvList []*kyverno.NamespacedPolicyViolation) error {
	newStatus := pc.calculateStatus(p.Name, pvList, nspvList)
	if reflect.DeepEqual(newStatus, p.Status) {
		// no update to status
		return nil
	}
	// update status
	newPolicy := p
	newPolicy.Status = newStatus
	_, err := pc.kyvernoClient.KyvernoV1().ClusterPolicies().UpdateStatus(newPolicy)
	return err
}

func (pc *PolicyController) calculateStatus(policyName string, pvList []*kyverno.ClusterPolicyViolation, nspvList []*kyverno.NamespacedPolicyViolation) kyverno.PolicyStatus {
	violationCount := len(pvList) + len(nspvList)
	status := kyverno.PolicyStatus{
		ViolationCount: violationCount,
	}
	// get stats
	stats := pc.statusAggregator.GetPolicyStats(policyName)
	if !reflect.DeepEqual(stats, (PolicyStatInfo{})) {
		status.RulesAppliedCount = stats.RulesAppliedCount
		status.ResourcesBlockedCount = stats.ResourceBlocked
		status.AvgExecutionTimeMutation = stats.MutationExecutionTime.String()
		status.AvgExecutionTimeValidation = stats.ValidationExecutionTime.String()
		status.AvgExecutionTimeGeneration = stats.GenerationExecutionTime.String()
		// update rule stats
		status.Rules = convertRules(stats.Rules)
	}
	return status
}

func (pc *PolicyController) getNamespacedPolicyViolationForPolicy(policy *kyverno.ClusterPolicy) ([]*kyverno.NamespacedPolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy.Name)
	if err != nil {
		return nil, err
	}
	// Get List of cluster policy violation
	nspvList, err := pc.nspvLister.List(policySelector)
	if err != nil {
		return nil, err
	}
	return nspvList, nil

}

//PVControlInterface provides interface to  operate on policy violation resource
type PVControlInterface interface {
	DeleteClusterPolicyViolation(name string) error
	DeleteNamespacedPolicyViolation(ns, name string) error
}

// RealPVControl is the default implementation of PVControlInterface.
type RealPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//DeletePolicyViolation deletes the policy violation
func (r RealPVControl) DeleteClusterPolicyViolation(name string) error {
	return r.Client.KyvernoV1().ClusterPolicyViolations().Delete(name, &metav1.DeleteOptions{})
}

//DeleteNamespacedPolicyViolation deletes the namespaced policy violation
func (r RealPVControl) DeleteNamespacedPolicyViolation(ns, name string) error {
	return r.Client.KyvernoV1().NamespacedPolicyViolations(ns).Delete(name, &metav1.DeleteOptions{})
}

// convertRules converts the internal rule stats to one used in policy.stats struct
func convertRules(rules []RuleStatinfo) []kyverno.RuleStats {
	var stats []kyverno.RuleStats
	for _, r := range rules {
		stat := kyverno.RuleStats{
			Name:           r.RuleName,
			ExecutionTime:  r.ExecutionTime.String(),
			AppliedCount:   r.RuleAppliedCount,
			ViolationCount: r.RulesFailedCount,
			MutationCount:  r.MutationCount,
		}
		stats = append(stats, stat)
	}
	return stats
}
