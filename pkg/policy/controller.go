package policy

import (
	"time"

	"github.com/go-logr/logr"
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
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
	nspvLister kyvernolister.PolicyViolationLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// pvListerSynced returns true if the Policy store has been synced at least once
	cpvListerSynced cache.InformerSynced
	// pvListerSynced returns true if the Policy Violation store has been synced at least once
	nspvListerSynced cache.InformerSynced
	// Resource manager, manages the mapping for already processed resource
	rm resourceManager
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// store to hold policy meta data for faster lookup
	pMetaStore policystore.UpdateInterface
	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface
	// resourceWebhookWatcher queues the webhook creation request, creates the webhook
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister
	log                    logr.Logger
}

// NewPolicyController create a new PolicyController
func NewPolicyController(kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	cpvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	nspvInformer kyvernoinformer.PolicyViolationInformer,
	configHandler config.Interface,
	eventGen event.Interface,
	pvGenerator policyviolation.GeneratorInterface,
	pMetaStore policystore.UpdateInterface,
	resourceWebhookWatcher *webhookconfig.ResourceWebhookRegister,
	log logr.Logger) (*PolicyController, error) {
	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Info)
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
		pMetaStore:             pMetaStore,
		pvGenerator:            pvGenerator,
		resourceWebhookWatcher: resourceWebhookWatcher,
		log:                    log,
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
	// resource manager
	// rebuild after 300 seconds/ 5 mins
	//TODO: pass the time in seconds instead of converting it internally
	pc.rm = NewResourceManager(30)

	return &pc, nil
}

func (pc *PolicyController) addPolicy(obj interface{}) {
	logger := pc.log
	p := obj.(*kyverno.ClusterPolicy)
	// Only process policies that are enabled for "background" execution
	// policy.spec.background -> "True"
	// register with policy meta-store
	pc.pMetaStore.Register(*p)

	// TODO: code might seem vague, awaiting resolution of issue https://github.com/nirmata/kyverno/issues/598
	if p.Spec.Background == nil {
		// if userInfo is not defined in policy we process the policy
		if err := ContainsUserInfo(*p); err != nil {
			return
		}
	} else {
		if !*p.Spec.Background {
			return
		}
		// If userInfo is used then skip the policy
		// ideally this should be handled by background flag only
		if err := ContainsUserInfo(*p); err != nil {
			// contains userInfo used in policy
			return
		}
	}
	logger.V(4).Info("adding policy", "name", p.Name)
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) updatePolicy(old, cur interface{}) {
	logger := pc.log
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)
	// TODO: optimize this : policy meta-store
	// Update policy-> (remove,add)
	err := pc.pMetaStore.UnRegister(*oldP)
	if err != nil {
		logger.Error(err, "failed to unregister policy", "name", oldP.Name)
	}
	pc.pMetaStore.Register(*curP)

	// Only process policies that are enabled for "background" execution
	// policy.spec.background -> "True"
	// TODO: code might seem vague, awaiting resolution of issue https://github.com/nirmata/kyverno/issues/598
	if curP.Spec.Background == nil {
		// if userInfo is not defined in policy we process the policy
		if err := ContainsUserInfo(*curP); err != nil {
			return
		}
	} else {
		if !*curP.Spec.Background {
			return
		}
		// If userInfo is used then skip the policy
		// ideally this should be handled by background flag only
		if err := ContainsUserInfo(*curP); err != nil {
			// contains userInfo used in policy
			return
		}
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
			logger.Info("couldnt get object from tomstone", "obj", obj)
			return
		}
		p, ok = tombstone.Obj.(*kyverno.ClusterPolicy)
		if !ok {
			logger.Info("tombstone container object that is not a policy", "obj", obj)
			return
		}
	}

	logger.V(4).Info("deleting policy", "name", p.Name)
	// Unregister from policy meta-store
	if err := pc.pMetaStore.UnRegister(*p); err != nil {
		logger.Error(err, "failed to unregister policy", "name", p.Name)
	}

	// we process policies that are not set of background processing as we need to perform policy violation
	// cleanup when a policy is deleted.
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) enqueue(policy *kyverno.ClusterPolicy) {
	logger := pc.log
	key, err := cache.MetaNamespaceKeyFunc(policy)
	if err != nil {
		logger.Error(err, "failed to enqueu policy")
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

	if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.cpvListerSynced, pc.nspvListerSynced) {
		logger.Info("failed to sync informer cache")
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, time.Second, stopCh)
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
	pc.registerResourceWebhookConfiguration()

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
		logger.V(4).Info("finished syncing policy", "key", key, "processingTime", time.Since(startTime))
	}()

	policy, err := pc.pLister.Get(key)
	if errors.IsNotFound(err) {
		go pc.deletePolicyViolations(key)

		// remove webhook configurations if there are no policies
		if err := pc.removeResourceWebhookConfiguration(); err != nil {
			// do not fail, if unable to delete resource webhook config
			logger.Error(err, "failed to remove resource webhook configurations")
		}

		return nil
	}

	if err != nil {
		return err
	}

	pc.resourceWebhookWatcher.RegisterResourceWebhook()

	engineResponses := pc.processExistingResources(*policy)
	pc.cleanupAndReport(engineResponses)

	return nil
}

func (pc *PolicyController) deletePolicyViolations(key string) {
	if err := pc.deleteClusterPolicyViolations(key); err != nil {
		pc.log.Error(err, "failed to delete policy violation", "key", key)
	}

	if err := pc.deleteNamespacedPolicyViolations(key); err != nil {
		pc.log.Error(err, "failed to delete policy violation", "key", key)
	}
}

func (pc *PolicyController) deleteClusterPolicyViolations(policy string) error {
	cpvList, err := pc.getClusterPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}

	for _, cpv := range cpvList {
		if err := pc.pvControl.DeleteClusterPolicyViolation(cpv.Name); err != nil {
			pc.log.Error(err, "failed to delete policy violation", "name", cpv.Name)
		}
	}

	return nil
}

func (pc *PolicyController) deleteNamespacedPolicyViolations(policy string) error {
	nspvList, err := pc.getNamespacedPolicyViolationForPolicy(policy)
	if err != nil {
		return err
	}

	for _, nspv := range nspvList {
		if err := pc.pvControl.DeleteNamespacedPolicyViolation(nspv.Namespace, nspv.Name); err != nil {
			pc.log.Error(err, "failed to delete policy violation", "name", nspv.Name)
		}
	}

	return nil
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
	DeleteNamespacedPolicyViolation(ns, name string) error
}

// RealPVControl is the default implementation of PVControlInterface.
type RealPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//DeleteClusterPolicyViolation deletes the policy violation
func (r RealPVControl) DeleteClusterPolicyViolation(name string) error {
	return r.Client.KyvernoV1().ClusterPolicyViolations().Delete(name, &metav1.DeleteOptions{})
}

//DeleteNamespacedPolicyViolation deletes the namespaced policy violation
func (r RealPVControl) DeleteNamespacedPolicyViolation(ns, name string) error {
	return r.Client.KyvernoV1().PolicyViolations(ns).Delete(name, &metav1.DeleteOptions{})
}
