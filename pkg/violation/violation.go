package violation

import (
	"encoding/json"
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	clientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	policyscheme "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions/policy/v1alpha1"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	//	"github.com/nirmata/kube-policy/webhooks"
	v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	patchTypes "k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcc1orev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

type Violations []Violation

type Violation struct {
}

// Mode for policy types
type Mode int

const (
	Mutate  Mode = 0
	Violate Mode = 1
)

// Info  input details
type Info struct {
	Resource string
	Policy   string
	rule     string
	Mode     Mode
	Reason   string
	crud     string
}

// Builder to generate violations
type Builder struct {
	kubeClient      *kubernetes.Clientset
	policyClientset *clientset.Clientset
	workqueue       workqueue.RateLimitingInterface
	logger          *log.Logger
	recorder        record.EventRecorder
	policyLister    lister.PolicyLister
	policySynced    cache.InformerSynced
}

func NewViolationHelper(kubeClient *kubernetes.Clientset, policyClientSet *clientset.Clientset, logger *log.Logger, policyInformer informers.PolicyInformer) (*Builder, error) {

	policyscheme.AddToScheme(scheme.Scheme)
	// Initialize Event Broadcaster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(
		&typedcc1orev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: "policy-controller"})

	builder := &Builder{
		kubeClient:      kubeClient,
		policyClientset: policyClientSet,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Policy-Violations"),
		logger:          logger,
		recorder:        recorder,
		policyLister:    policyInformer.Lister(),
		policySynced:    policyInformer.Informer().HasSynced,
	}
	return builder, nil
}

// Create to generate violation jsonpatch script &
// queue events to generate events
// TO-DO create should validate the rule number and update the violation if one exists
func (b *Builder) Create(info Info) ([]byte, error) {

	// generate patch
	patchBytes, err := b.generateViolationPatch(info)
	if err != nil {
		return nil, err
	}
	// generate event
	// add to queue
	b.workqueue.Add(info)
	return patchBytes, nil
}

func (b *Builder) Remove(info Info) ([]byte, error) {
	b.workqueue.Add(info)
	return nil, nil
}

func (b *Builder) generateViolationPatch(info Info) ([]byte, error) {
	// policy-controller handlers are post events
	// adm-ctr will always have policy resource created
	// Get Policy namespace and name
	policyNamespace, policyName, err := cache.SplitMetaNamespaceKey(info.Policy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", info.Policy))
		return nil, err
	}
	// Try to access the policy
	// Try to access the resource
	// if the above resource objects have not been created then we reque the request to create the event
	policy, err := b.policyLister.Policies(policyNamespace).Get(policyName)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	// Add violation
	updatedPolicy := policy.DeepCopy()
	updatedPolicy.Status.Logs = append(updatedPolicy.Status.Logs, info.Reason)
	return b.patch(policy, updatedPolicy)
}

func (b *Builder) patch(policy *types.Policy, updatedPolicy *types.Policy) ([]byte, error) {
	originalPolicy, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}
	modifiedPolicy, err := json.Marshal(updatedPolicy)
	if err != nil {
		return nil, err
	}
	patchBytes, err := jsonpatch.CreateMergePatch(originalPolicy, modifiedPolicy)
	if err != nil {
		return nil, err
	}
	return patchBytes, nil
	// _, err = b.PolicyClientset.Nirmata().Policies(policy.Namespace).Patch(policy.Name, patchTypes.MergePatchType, patchBytes)
	// if err != nil {
	// 	return err
	// }
	// return nil

}

func (b *Builder) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer b.workqueue.ShutDown()
	log.Println("Starting violation builder")

	fmt.Println(("Wait for informer cache to sync"))
	if ok := cache.WaitForCacheSync(stopCh, b.policySynced); !ok {
		fmt.Println("Unable to sync the cache")
	}

	log.Println("Starting workers")
	for i := 0; i < threadiness; i++ {
		go wait.Until(b.runWorker, time.Second, stopCh)
	}
	log.Println("Started workers")
	<-stopCh
	log.Println("Shutting down workers")
	return nil
}

func (b *Builder) runWorker() {
	for b.processNextWorkItem() {
	}
}

func (b *Builder) processNextWorkItem() bool {
	// get info object
	obj, shutdown := b.workqueue.Get()
	if shutdown {
		return false
	}
	err := func(obj interface{}) error {
		defer b.workqueue.Done(obj)

		var key Info
		var ok bool
		if key, ok = obj.(Info); !ok {
			b.workqueue.Forget(obj)
			log.Printf("Expecting type info by got %v", obj)
			return nil
		}

		// Run the syncHandler, passing the resource and the policy
		if err := b.syncHandler(key); err != nil {
			b.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' & '%s': %s, requeuing", key.Resource, key.Policy, err.Error())
		}

		return nil
	}(obj)

	if err != nil {
		log.Println((err))
	}
	return true

}

// TO-DO: how to handle events if the resource has been delted, and clean the dirty object
func (b *Builder) syncHandler(key Info) error {

	// Get Policy namespace and name
	policyNamespace, policyName, err := cache.SplitMetaNamespaceKey(key.Policy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key.Policy))
		return nil
	}

	// Try to access the policy
	// Try to access the resource
	// if the above resource objects have not been created then we reque the request to create the event
	fmt.Println(policyNamespace)
	fmt.Println(policyName)

	policy, err := b.policyLister.Policies(policyNamespace).Get(policyName)
	if err != nil {
		fmt.Println(err)
		return err
	}
	resourceNamespace, resourceName, err := cache.SplitMetaNamespaceKey(key.Resource)
	fmt.Println(resourceNamespace)
	fmt.Println(resourceName)

	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key.Resource))
		return nil
	}

	// Get Resource namespace and name
	resource, err := b.kubeClient.AppsV1().Deployments(resourceNamespace).Get(resourceName, meta_v1.GetOptions{}) // Deployment
	if err != nil {
		fmt.Println(err)
		return err
	}

	// Generate events for policy
	b.recorder.Event(policy, v1.EventTypeNormal, "violation", key.Reason)

	// Generate events for resource
	b.recorder.Event(resource, v1.EventTypeNormal, "violation", key.Reason)

	return nil
}
