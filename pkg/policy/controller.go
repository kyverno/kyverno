package policy

import (
	"fmt"
	policyreportinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/policyreport/v1alpha1"
	"github.com/nirmata/kyverno/pkg/policyreport"
	"os"
	"strings"
	"time"

	informers "k8s.io/client-go/informers/core/v1"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/config"
	"github.com/nirmata/kyverno/pkg/constant"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
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
	eventGen      event.Interface
	eventRecorder record.EventRecorder

	//pvControl is used for adoptin/releasing policy violation
	pvControl PVControlInterface

	// Policys that need to be synced
	queue workqueue.RateLimitingInterface

	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister

	// npLister can list/get namespace policy from the shared informer's store
	npLister kyvernolister.PolicyLister

	// pvLister can list/get policy violation from the shared informer's store
	cpvLister kyvernolister.ClusterPolicyViolationLister

	// nspvLister can list/get namespaced policy violation from the shared informer's store
	nspvLister kyvernolister.PolicyViolationLister

	// nsLister can list/get namespacecs from the shared informer's store
	nsLister listerv1.NamespaceLister

	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced

	// npListerSynced returns true if the Policy store has been synced at least once
	npListerSynced cache.InformerSynced

	// pvListerSynced returns true if the Policy store has been synced at least once
	cpvListerSynced cache.InformerSynced

	// pvListerSynced returns true if the Policy Violation store has been synced at least once
	nspvListerSynced cache.InformerSynced

	// nsListerSynced returns true if the namespace store has been synced at least once
	nsListerSynced cache.InformerSynced

	// Resource manager, manages the mapping for already processed resource
	rm resourceManager

	// helpers to validate against current loaded configuration
	configHandler config.Interface

	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface

	// resourceWebhookWatcher queues the webhook creation request, creates the webhook
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister

	policyInformer policyreportinformer.Interface
	prgen          policyreport.Generator

	log logr.Logger
}

// NewPolicyController create a new PolicyController
func NewPolicyController(kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	policyInformer policyreportinformer.Interface,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	npInformer kyvernoinformer.PolicyInformer,
	cpvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	nspvInformer kyvernoinformer.PolicyViolationInformer,
	configHandler config.Interface, eventGen event.Interface,
	pvGenerator policyviolation.GeneratorInterface,
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister,
	namespaces informers.NamespaceInformer,
	log logr.Logger) (*PolicyController, error) {

	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.V(5).Info)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		return nil, err
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pc := PolicyController{
		client:                 client,
		kyvernoClient:          kyvernoClient,
		eventGen:               eventGen,
		eventRecorder:          eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "policy_controller"}),
		queue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policy"),
		configHandler:          configHandler,
		pvGenerator:            pvGenerator,
		resourceWebhookWatcher: resourceWebhookWatcher,
		log:                    log,
		policyInformer:  policyInformer,
		prgen:                  policyreport.Generator{},
	}

	pc.pvControl = RealPVControl{Client: kyvernoClient, Recorder: pc.eventRecorder}

	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})
	// Policy informer event handler
	npInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addNsPolicy,
		UpdateFunc: pc.updateNsPolicy,
		DeleteFunc: pc.deleteNsPolicy,
	})

	if os.Getenv("POLICY-TYPE") != "POLICYREPORT" {
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
		pc.cpvLister = cpvInformer.Lister()
		pc.nspvLister = nspvInformer.Lister()
		pc.cpvListerSynced = cpvInformer.Informer().HasSynced
		pc.nspvListerSynced = nspvInformer.Informer().HasSynced
	}

	pc.pLister = pInformer.Lister()
	pc.nsLister = namespaces.Lister()
	pc.pListerSynced = pInformer.Informer().HasSynced
	pc.nsListerSynced = namespaces.Informer().HasSynced


	// resource manager
	// rebuild after 300 seconds/ 5 mins
	//TODO: pass the time in seconds instead of converting it internally
	pc.rm = NewResourceManager(30)

	return &pc, nil
}

func (pc *PolicyController) canBackgroundProcess(p *kyverno.ClusterPolicy) bool {
	logger := pc.log.WithValues("policy", p.Name)
	if !p.BackgroundProcessingEnabled() {
		logger.V(4).Info("background processed is disabled")
		return false
	}

	if err := ContainsVariablesOtherThanObject(*p); err != nil {
		logger.V(4).Info("policy cannot be processed in the background")
		return false
	}

	return true
}

func (pc *PolicyController) addPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyverno.ClusterPolicy)
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

	if !pc.canBackgroundProcess(curP) {
		return
	}

	logger.V(4).Info("updating policy", "name", oldP.Name)
	pc.enqueuePolicy(curP)
}

func (pc *PolicyController) deletePolicy(obj interface{}) {

	logger := pc.log
	p, ok := obj.(*kyverno.ClusterPolicy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldn't get object from tomstone", "obj", obj)
			return
		}

		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			logger.Info("tombstone container object that is not a policy", "obj", obj)
			return
		}
	}

	logger.V(4).Info("deleting policy", "name", p.Name)

	// we process policies that are not set of background processing as we need to perform policy violation
	// cleanup when a policy is deleted.
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) addNsPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyverno.Policy)
	pol := convertPolicyToClusterPolicy(p)
	if !pc.canBackgroundProcess(pol) {
		return
	}
	logger.V(4).Info("queuing policy for background processing", "namespace", pol.Namespace, "name", pol.Name)
	pc.enqueuePolicy(pol)
}

func (pc *PolicyController) updateNsPolicy(old, cur interface{}) {
	logger := pc.log
	oldP := old.(*kyverno.Policy)
	curP := cur.(*kyverno.Policy)
	ncurP := convertPolicyToClusterPolicy(curP)
	if !pc.canBackgroundProcess(ncurP) {
		return
	}

	logger.V(4).Info("updating namespace policy", "namespace", oldP.Namespace, "name", oldP.Name)
	pc.enqueuePolicy(ncurP)
}

func (pc *PolicyController) deleteNsPolicy(obj interface{}) {
	logger := pc.log
	p, ok := obj.(*kyverno.Policy)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			logger.Info("couldnt get object from tomstone", "obj", obj)
			return
		}

		p, ok = tombstone.Obj.(*kyverno.Policy)
		if !ok {
			logger.Info("tombstone container object that is not a policy", "obj", obj)
			return
		}
	}
	pol := convertPolicyToClusterPolicy(p)
	logger.V(4).Info("deleting namespace policy", "namespace", pol.Namespace, "name", pol.Name)

	// we process policies that are not set of background processing as we need to perform policy violation
	// cleanup when a policy is deleted.
	pc.enqueuePolicy(pol)
}

func (pc *PolicyController) enqueuePolicy(policy *kyverno.ClusterPolicy) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueue policy")
		return
	}
	pc.queue.Add(key)
}

// Run begins watching and syncing.
func (pc *PolicyController) Run(workers int, stopCh <-chan struct{}) {
	logger := pc.log

	defer utilruntime.HandleCrash()
	defer pc.queue.ShutDown()

	logger.Info("starting")
	defer logger.Info("shutting down")

	if os.Getenv("POLICY-TYPE") != "POLICYREPORT" {
		if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.nsListerSynced, pc.nspvListerSynced, pc.cpvListerSynced) {
			logger.Info("failed to sync informer cache")
			return
		}
	} else {
		if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.nsListerSynced) {
			logger.Info("failed to sync informer cache")
			return
		}
	}

	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, constant.PolicyControllerResync, stopCh)
	}

	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pc *PolicyController) worker() {
	for pc.processNextWorkItem() {
	}
}

func (pc *PolicyController) processNextWorkItem() bool {
	// if policies exist before Kyverno get created, resource webhook configuration
	// could not be registered as clusterpolicy.spec.background=false by default
	// the policy controller would starts only when the first incoming policy is queued
	pc.resourceWebhookWatcher.RegisterResourceWebhook()

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
	logger := pc.log
	startTime := time.Now()
	logger.V(4).Info("started syncing policy", "key", key, "startTime", startTime)
	defer func() {
		logger.V(4).Info("finished syncing policy", "key", key, "processingTime", time.Since(startTime).String())
	}()

	namespace, key, isNamespacedPolicy := getIsNamespacedPolicy(key)
	var policy *kyverno.ClusterPolicy
	var err error
	if !isNamespacedPolicy {
		policy, err = pc.pLister.Get(key)
	} else {
		var nspolicy *kyverno.Policy
		nspolicy, err = pc.npLister.Policies(namespace).Get(key)
		policy = convertPolicyToClusterPolicy(nspolicy)
	}
	if errors.IsNotFound(err) {

		go pc.deletePolicyViolations(key)

		// remove webhook configurations if there are no policies
		if err := pc.removeResourceWebhookConfiguration(); err != nil {
			logger.Error(err, "failed to remove resource webhook configurations")
		}

		return nil
	}

	if err != nil {
		return err
	}

	pc.resourceWebhookWatcher.RegisterResourceWebhook()

	engineResponses := pc.processExistingResources(policy)
	pc.cleanupAndReport(engineResponses)
	return nil
}

func (pc *PolicyController) deletePolicyViolations(key string) {
	cpv, err := pc.deleteClusterPolicyViolations(key)
	if err != nil {
		pc.log.Error(err, "failed to delete policy violations", "policy", key)
	}

	npv, err := pc.deleteNamespacedPolicyViolations(key)
	if err != nil {
		pc.log.Error(err, "failed to delete policy violations", "policy", key)
	}

	pc.log.Info("deleted policy violations", "policy", key, "count", cpv+npv)
}

func (pc *PolicyController) deleteClusterPolicyViolations(policy string) (int, error) {
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		str := strings.Split(policy, "/")
		if err := pc.pvControl.DeleteClusterPolicyViolation(str[0]); err != nil {
			pc.log.Error(err, "failed to delete policy violation", "name", policy)
		}
		return 0,nil
	}
	cpvList, err := pc.getClusterPolicyViolationForPolicy(policy)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, cpv := range cpvList {
		if err := pc.pvControl.DeleteClusterPolicyViolation(policy); err != nil {
			pc.log.Error(err, "failed to delete policy violation", "name", cpv.Name)
		} else {
			count++
		}
	}
	return count, nil
}

func (pc *PolicyController) deleteNamespacedPolicyViolations(policy string) (int, error) {
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		str := strings.Split(policy, "/")
		if len(str) == 2 {
			if err := pc.pvControl.DeleteNamespacedPolicyViolation(str[0], str[2]); err != nil {
				pc.log.Error(err, "failed to delete policy violation", "name", str[0])
			}
		}
		return 0,nil
	}
	nspvList, err := pc.getNamespacedPolicyViolationForPolicy(policy)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, nspv := range nspvList {
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(policy, nspv.Name); err != nil {
			pc.log.Error(err, "failed to delete policy violation", "name", nspv.Name)
		} else {
			count++
		}
	}

	return count, nil
}

func (pc *PolicyController) getNamespacedPolicyViolationForPolicy(policy string) ([]*kyverno.PolicyViolation, error) {
	policySelector, err := buildPolicyLabel(policy)
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
	DeleteNamespacedPolicyViolation(name, ns string) error
}

// RealPVControl is the default implementation of PVControlInterface.
type RealPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//DeleteClusterPolicyViolation deletes the policy violation
func (r RealPVControl) DeleteClusterPolicyViolation(name string) error {
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		policyReport, err := r.Client.PolicyV1alpha1().ClusterPolicyReports().Get("kyverno-clusterpolicyreport", metav1.GetOptions{})
		if err != nil {
			if !errors.IsNotFound(err) {
				return err
			}
			return nil
		}
		policyReport = policyreport.RemoveClusterPolicyViolation(policyReport, name)
		_, err = r.Client.PolicyV1alpha1().ClusterPolicyReports().Update(policyReport)
		return err
	}
	err := r.Client.KyvernoV1().ClusterPolicyViolations().Delete(name, &metav1.DeleteOptions{})
	return err
}

//DeleteNamespacedPolicyViolation deletes the namespaced policy violation
func (r RealPVControl) DeleteNamespacedPolicyViolation(name, ns string) error {
	if os.Getenv("POLICY-TYPE") == "POLICYREPORT" {
		reportName := fmt.Sprintf("kyverno-clusterpolicyreport-%s", ns)
		policyReport, err := r.Client.PolicyV1alpha1().PolicyReports(ns).Get(reportName, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		policyReport = policyreport.RemovePolicyViolation(policyReport, name)
		_, err = r.Client.PolicyV1alpha1().PolicyReports(ns).Update(policyReport)
		return err
	}
	return r.Client.KyvernoV1().PolicyViolations(ns).Delete(name, &metav1.DeleteOptions{})
}
