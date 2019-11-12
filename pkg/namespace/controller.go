package namespace

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/policystore"
	"github.com/nirmata/kyverno/pkg/policyviolation"
	"k8s.io/apimachinery/pkg/api/errors"

	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	v1Informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// maxRetries is the number of times a Namespace will be processed for a policy before its dropped from the queue
	maxRetries = 15
)

//NamespaceController watches the 'Namespace' resource creation/update and applied the generation rules on them
type NamespaceController struct {
	client        *client.Client
	kyvernoClient *kyvernoclient.Clientset
	syncHandler   func(nsKey string) error
	enqueueNs     func(ns *v1.Namespace)

	//nsLister provides expansion to the namespace lister to inject GVK for the resource
	nsLister NamespaceListerExpansion
	// nLsister can list/get namespaces from the shared informer's store
	// nsLister v1CoreLister.NamespaceLister
	// nsListerSynced returns true if the Namespace store has been synced at least once
	nsListerSynced cache.InformerSynced
	// pvLister can list/get policy violation from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// pvListerSynced retrns true if the Policy store has been synced at least once
	pvListerSynced cache.InformerSynced
	// pvLister can list/get policy violation from the shared informer's store
	pvLister kyvernolister.ClusterPolicyViolationLister
	// API to send policy stats for aggregation
	policyStatus policy.PolicyStatusInterface
	// eventGen provides interface to generate evenets
	eventGen event.Interface
	// Namespaces that need to be synced
	queue workqueue.RateLimitingInterface
	// Resource manager, manages the mapping for already processed resource
	rm resourceManager
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// store to hold policy meta data for faster lookup
	pMetaStore policystore.LookupInterface
	// policy violation generator
	pvGenerator policyviolation.GeneratorInterface
}

//NewNamespaceController returns a new Controller to manage generation rules
func NewNamespaceController(kyvernoClient *kyvernoclient.Clientset,
	client *client.Client,
	nsInformer v1Informer.NamespaceInformer,
	pInformer kyvernoinformer.ClusterPolicyInformer,
	pvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	policyStatus policy.PolicyStatusInterface,
	eventGen event.Interface,
	configHandler config.Interface,
	pvGenerator policyviolation.GeneratorInterface,
	pMetaStore policystore.LookupInterface) *NamespaceController {
	//TODO: do we need to event recorder for this controller?
	// create the controller
	nsc := &NamespaceController{
		client:        client,
		kyvernoClient: kyvernoClient,
		eventGen:      eventGen,
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "namespace"),
		configHandler: configHandler,
		pMetaStore:    pMetaStore,
		pvGenerator:   pvGenerator,
	}

	nsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nsc.addNamespace,
		UpdateFunc: nsc.updateNamespace,
		DeleteFunc: nsc.deleteNamespace,
	})

	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    nsc.addPolicy,
		UpdateFunc: nsc.updatePolicy,
	})

	nsc.enqueueNs = nsc.enqueue
	nsc.syncHandler = nsc.syncNamespace

	nsc.nsLister = NewNamespaceLister(nsInformer.Lister())
	nsc.nsListerSynced = nsInformer.Informer().HasSynced
	nsc.pLister = pInformer.Lister()
	nsc.pvListerSynced = pInformer.Informer().HasSynced
	nsc.pvLister = pvInformer.Lister()
	nsc.policyStatus = policyStatus

	// resource manager
	// rebuild after 300 seconds/ 5 mins
	nsc.rm = NewResourceManager(300)

	return nsc
}
func (nsc *NamespaceController) addPolicy(obj interface{}) {
	p := obj.(*kyverno.ClusterPolicy)
	// check if the policy has generate rule
	if generateRuleExists(p) {
		// process policy
		nsc.processPolicy(p)
	}
}

func (nsc *NamespaceController) updatePolicy(old, cur interface{}) {
	curP := cur.(*kyverno.ClusterPolicy)
	// check if the policy has generate rule
	if generateRuleExists(curP) {
		// process policy
		nsc.processPolicy(curP)
	}
}

func (nsc *NamespaceController) addNamespace(obj interface{}) {
	ns := obj.(*v1.Namespace)
	glog.V(4).Infof("Adding Namespace %s", ns.Name)
	nsc.enqueueNs(ns)
}

func (nsc *NamespaceController) updateNamespace(old, cur interface{}) {
	oldNs := old.(*v1.Namespace)
	curNs := cur.(*v1.Namespace)
	if curNs.ResourceVersion == oldNs.ResourceVersion {
		// Periodic resync will send update events for all known Namespace.
		// Two different versions of the same replica set will always have different RVs.
		return
	}
	glog.V(4).Infof("Updating Namesapce %s", curNs.Name)
	//TODO: anything to be done here?
}

func (nsc *NamespaceController) deleteNamespace(obj interface{}) {
	ns, _ := obj.(*v1.Namespace)
	glog.V(4).Infof("Deleting Namespace %s", ns.Name)
	//TODO: anything to be done here?
}

func (nsc *NamespaceController) enqueue(ns *v1.Namespace) {
	key, err := cache.MetaNamespaceKeyFunc(ns)
	if err != nil {
		glog.Error(err)
		return
	}
	nsc.queue.Add(key)
}

//Run to run the controller
func (nsc *NamespaceController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer nsc.queue.ShutDown()

	glog.Info("Starting namespace controller")
	defer glog.Info("Shutting down namespace controller")

	if ok := cache.WaitForCacheSync(stopCh, nsc.nsListerSynced); !ok {
		return
	}

	for i := 0; i < workerCount; i++ {
		go wait.Until(nsc.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (nsc *NamespaceController) worker() {
	for nsc.processNextWorkItem() {
	}
}

func (nsc *NamespaceController) processNextWorkItem() bool {
	key, quit := nsc.queue.Get()
	if quit {
		return false
	}
	defer nsc.queue.Done(key)

	err := nsc.syncHandler(key.(string))
	nsc.handleErr(err, key)

	return true
}

func (nsc *NamespaceController) handleErr(err error, key interface{}) {
	if err == nil {
		nsc.queue.Forget(key)
		return
	}

	if nsc.queue.NumRequeues(key) < maxRetries {
		glog.V(2).Infof("Error syncing namespace %v: %v", key, err)
		nsc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(2).Infof("Dropping namespace %q out of the queue: %v", key, err)
	nsc.queue.Forget(key)
}

func (nsc *NamespaceController) syncNamespace(key string) error {
	startTime := time.Now()
	glog.V(4).Infof("Started syncing namespace %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing namespace %q (%v)", key, time.Since(startTime))
	}()
	namespace, err := nsc.nsLister.GetResource(key)
	if errors.IsNotFound(err) {
		glog.V(2).Infof("namespace %v has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}
	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	n := namespace.DeepCopy()

	// skip processing namespace if its been filtered
	// exclude the filtered resources
	if nsc.configHandler.ToFilter("Namespace", "", namespace.Name) {
		//TODO: improve the text
		glog.V(4).Infof("excluding namespace %s as its a filtered resource", namespace.Name)
		return nil
	}

	// process generate rules
	engineResponses := nsc.processNamespace(*n)
	// report errors
	nsc.report(engineResponses)
	return nil
}
