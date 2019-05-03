package violation

import (
	"fmt"
	"log"
	"time"

	types "github.com/nirmata/kube-policy/pkg/apis/policy/v1alpha1"
	clientset "github.com/nirmata/kube-policy/pkg/client/clientset/versioned"
	policyscheme "github.com/nirmata/kube-policy/pkg/client/clientset/versioned/scheme"
	informers "github.com/nirmata/kube-policy/pkg/client/informers/externalversions/policy/v1alpha1"
	lister "github.com/nirmata/kube-policy/pkg/client/listers/policy/v1alpha1"
	resourceClient "github.com/nirmata/kube-policy/pkg/resourceClient"
	v1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcc1orev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type Violations []Violation

type Violation struct {
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

	// Initialize Event Broadcaster
	policyscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Printf)
	eventBroadcaster.StartRecordingToSink(
		&typedcc1orev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: violationEventSource})
	// Build the builder
	builder := &Builder{
		kubeClient:      kubeClient,
		policyClientset: policyClientSet,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workqueueViolationName),
		logger:          logger,
		recorder:        recorder,
		policyLister:    policyInformer.Lister(),
		policySynced:    policyInformer.Informer().HasSynced,
	}
	return builder, nil
}

// Create Violation -> (Info)

// Create to generate violation jsonpatch script &
// queue events to generate events
// TODO: create should validate the rule number and update the violation if one exists
func (b *Builder) Create(info Info) error {
	// generate patch
	//	we can generate the patch as the policy resource will alwasy exist
	// Apply Patch
	err := b.patchViolation(info)
	if err != nil {
		return err
	}

	// Generate event for policy
	b.workqueue.Add(
		EventInfo{
			Resource:       info.Policy,
			Reason:         info.Reason,
			ResourceTarget: PolicyTarget,
		})
	// Generat event for resource
	b.workqueue.Add(
		EventInfo{
			Kind:           info.Kind,
			Resource:       info.Resource,
			Reason:         info.Reason,
			ResourceTarget: ResourceTarget,
		})

	return nil
}

// Remove the violation
func (b *Builder) Remove(info Info) ([]byte, error) {
	b.workqueue.Add(info)
	return nil, nil
}

func (b *Builder) patchViolation(info Info) error {
	// policy-controller handlers are post events
	// adm-ctr will always have policy resource created
	// Get Policy namespace and name
	policyNamespace, policyName, err := cache.SplitMetaNamespaceKey(info.Policy)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", info.Policy))
		return err
	}
	// Try to access the policy
	// Try to access the resource
	// if the above resource objects have not been created then we reque the request to create the event
	policy, err := b.policyLister.Policies(policyNamespace).Get(policyName)
	if err != nil {
		utilruntime.HandleError(err)
		return err
	}
	// Add violation
	updatedPolicy := policy.DeepCopy()
	// var update bool
	// inactiveViolationindex := []int{}
	updatedViolations := []types.Violation{}
	// Check if the violation with the same rule exists for the same resource and rule name
	for _, violation := range updatedPolicy.Status.Violations {

		if ok, err := b.IsActive(violation); ok {
			if err != nil {
				fmt.Println(err)
			}
			updatedViolations = append(updatedViolations, violation)
		} else {
			fmt.Println("Remove violation")
			b.workqueue.Add(
				EventInfo{
					Resource:       info.Policy,
					Reason:         "Removing violation for rule " + info.RuleName,
					ResourceTarget: PolicyTarget,
				})
		}
	}
	// Rule is updated TO-DO
	// Dont validate if the resouce is active as a new Violation will not be created if it did not
	updatedViolations = append(updatedViolations,
		types.Violation{
			Kind:     info.Kind,
			Resource: info.Resource,
			Rule:     info.RuleName,
			Reason:   info.Reason,
		})
	updatedPolicy.Status.Violations = updatedViolations
	// Patch
	return b.patch(policy, updatedPolicy)
}

func (b *Builder) getPolicyEvent(info Info) EventInfo {
	return EventInfo{Resource: info.Resource}
}

func (b *Builder) IsActive(violation types.Violation) (bool, error) {
	if ok, err := b.ValidationResourceActive(violation); !ok {
		return false, err
	}
	return true, nil
}

func (b *Builder) ValidationResourceActive(violation types.Violation) (bool, error) {
	resourceNamespace, resourceName, err := cache.SplitMetaNamespaceKey(violation.Resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", violation.Resource))
		// Remove the corresponding violation
		return false, err
	}

	// Check if the corresponding resource is still present
	_, err = resourceClient.GetResouce(b.kubeClient, violation.Kind, resourceNamespace, resourceName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get resource %s ", violation.Resource))
		return false, err
	}

	return true, nil
}

func (b *Builder) patch(policy *types.Policy, updatedPolicy *types.Policy) error {
	_, err := b.policyClientset.Nirmata().Policies(updatedPolicy.Namespace).UpdateStatus(updatedPolicy)
	if err != nil {
		return err
	}
	return nil
}

// Run : Initialize the worker routines to process the event creation
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
		var key EventInfo
		var ok bool
		if key, ok = obj.(EventInfo); !ok {
			b.workqueue.Forget(obj)
			log.Printf("Expecting type info but got %v", obj)
			return nil
		}

		// Run the syncHandler, passing the resource and the policy
		if err := b.syncHandler(key); err != nil {
			b.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s' : %s, requeuing event creation request", key.Resource, err.Error())
		}

		return nil
	}(obj)

	if err != nil {
		log.Println((err))
	}
	return true

}

// TO-DO: how to handle events if the resource has been delted, and clean the dirty object
func (b *Builder) syncHandler(key EventInfo) error {
	fmt.Println(key)
	// Get Policy namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key.Resource)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid policy key: %s", key.Resource))
		return nil
	}
	if key.ResourceTarget == ResourceTarget {
		// Resource Event
		resource, err := resourceClient.GetResouce(b.kubeClient, key.Kind, namespace, name)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for resource %s, will retry ", key.Resource))
			return err
		}
		b.recorder.Event(resource, v1.EventTypeNormal, violationEventResrouce, key.Reason)
	} else {
		// Policy Event
		policy, err := b.policyLister.Policies(namespace).Get(name)
		if err != nil {
			// TO-DO: this scenario will not exist as the policy will always exist
			// unless the namespace and resource name are invalid
			utilruntime.HandleError(err)
			return err
		}
		b.recorder.Event(policy, v1.EventTypeNormal, violationEventResrouce, key.Reason)
	}

	return nil
}
