package violation

import (
	"fmt"
	"time"

	"github.com/go-logr/logr"
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
	"k8s.io/klog"
	"k8s.io/klog/klogr"
)

type Violations []Violation

type Violation struct {
}

// Builder to generate violations
type Builder struct {
	kubeClient      *kubernetes.Clientset
	policyClientset *clientset.Clientset
	workqueue       workqueue.RateLimitingInterface
	recorder        record.EventRecorder
	logger          logr.Logger
	policyLister    lister.PolicyLister
	policySynced    cache.InformerSynced
}

func NewViolationHelper(kubeClient *kubernetes.Clientset, policyClientSet *clientset.Clientset, policyInformer informers.PolicyInformer) (*Builder, error) {

	logger := klogr.New().WithName("Violation Builder ")

	logger.V(5).Info("Initialize Event Broadcaster")
	policyscheme.AddToScheme(scheme.Scheme)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcc1orev1.EventSinkImpl{
			Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		v1.EventSource{Component: violationEventSource})

	logger.V(5).Info("Build the builder")
	builder := &Builder{
		kubeClient:      kubeClient,
		policyClientset: policyClientSet,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workqueueViolationName),
		recorder:        recorder,
		logger:          logger,
		policyLister:    policyInformer.Lister(),
		policySynced:    policyInformer.Informer().HasSynced,
	}
	return builder, nil
}

// Create Violation -> (Info)

// Create to generate violation jsonpatch script &
// queue events to generate events
// TO-DO create should validate the rule number and update the violation if one exists
func (b *Builder) Create(info Info) error {
	b.logger.V(5).Info("generate patch")
	err := b.patchViolation(info)
	if err != nil {
		return err
	}

	b.logger.V(5).Info(fmt.Sprintf("generate event for policy %s", info.Policy))
	b.workqueue.Add(
		EventInfo{
			Resource:       info.Policy,
			Reason:         info.Reason,
			ResourceTarget: PolicyTarget,
		})
	b.logger.V(5).Info(fmt.Sprintf("generate event for resource %s", info.Resource))
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
	b.logger.V(5).Info(fmt.Sprintf("try to get policy %s", info.Policy))
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
	b.logger.V(5).Info("update violations")
	var updateViolation bool
	// Check if the violation with the same rule exists for the same resource and rule name
	for _, violation := range updatedPolicy.Status.Violations {

		if ok, err := b.IsActive(violation); ok {
			if err != nil {
				utilruntime.HandleError(err)
				continue
			}
			updatedViolations = append(updatedViolations, violation)
		} else {
			b.logger.V(5).Info(fmt.Sprintf("remove violation %v", violation))
			b.workqueue.Add(
				EventInfo{
					Resource:       info.Policy,
					Reason:         "Removing violation for rule " + info.RuleName,
					ResourceTarget: PolicyTarget,
				})
		}
		// Check if the voilation exists for the rule
		if violation.Kind == info.Kind &&
			violation.Resource == info.Resource &&
			violation.Rule == info.RuleName {

			b.logger.V(5).Info(fmt.Sprintf("update violations reason for rule %s, from %s to %s", violation.Rule, violation.Reason, info.Reason))
			violation.Reason = info.Reason
			updatedViolations = append(updatedViolations, violation)
			updateViolation = true
		}
	}
	if !updateViolation {
		b.logger.V(5).Info(fmt.Sprintf("adding new violation for rule %s, %s", info.RuleName, info.Reason))
		updatedViolations = append(updatedViolations,
			types.Violation{
				Kind:     info.Kind,
				Resource: info.Resource,
				Rule:     info.RuleName,
				Reason:   info.Reason,
			})
	}
	updatedPolicy.Status.Violations = updatedViolations
	// Patch
	return b.patch(policy, updatedPolicy)
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
		return false, err
	}
	b.logger.V(5).Info(fmt.Sprintf("check if resource %s exists!", violation.Reason))
	// Check if the corresponding resource is still present
	_, err = resourceClient.GetResouce(b.kubeClient, violation.Kind, resourceNamespace, resourceName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to get resource %s ", violation.Resource))
		return false, err
	}
	return true, nil
}

func (b *Builder) patch(policy *types.Policy, updatedPolicy *types.Policy) error {

	b.logger.V(5).Info("update violations!")
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
	b.logger.Info("Starting violation builder")

	b.logger.Info("Wait for informer cache to sync")
	if ok := cache.WaitForCacheSync(stopCh, b.policySynced); !ok {
		b.logger.Error(fmt.Errorf("Unable to sync the cache"), "")
	}
	b.logger.Info(fmt.Sprintf("Starting %d workers to process violation events", threadiness))
	for i := 0; i < threadiness; i++ {
		go wait.Until(b.runWorker, time.Second, stopCh)
	}
	b.logger.Info("Started workers")
	<-stopCh
	b.logger.Info("Shutting down workers")
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
			b.logger.Error(fmt.Errorf("Expecting type info by got %v", obj), "")
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
		b.logger.Error(err, "")
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
		b.logger.Error(fmt.Errorf("process event of type resource for %s", key.Resource), "")
		resource, err := resourceClient.GetResouce(b.kubeClient, key.Kind, namespace, name)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("unable to create event for resource %s, will retry ", key.Resource))
			return err
		}
		b.recorder.Event(resource, v1.EventTypeNormal, violationEventResrouce, key.Reason)
	} else {
		// Policy Event
		b.logger.Error(fmt.Errorf("process event of type policy for %s", key.Resource), "")
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
