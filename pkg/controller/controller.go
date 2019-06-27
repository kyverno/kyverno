package controller

import (
	"fmt"
	"strings"
	"time"

	"github.com/nirmata/kyverno/pkg/info"

	"github.com/nirmata/kyverno/pkg/engine"

	"github.com/golang/glog"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	lister "github.com/nirmata/kyverno/pkg/client/listers/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/sharedinformer"
	violation "github.com/nirmata/kyverno/pkg/violation"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

//PolicyController to manage Policy CRD
type PolicyController struct {
	client           *client.Client
	policyLister     lister.PolicyLister
	policySynced     cache.InformerSynced
	violationBuilder violation.Generator
	eventController  event.Generator
	queue            workqueue.RateLimitingInterface
}

// NewPolicyController from cmd args
func NewPolicyController(client *client.Client,
	policyInformer sharedinformer.PolicyInformer,
	violationBuilder violation.Generator,
	eventController event.Generator) *PolicyController {

	controller := &PolicyController{
		client:           client,
		policyLister:     policyInformer.GetLister(),
		policySynced:     policyInformer.GetInfomer().HasSynced,
		violationBuilder: violationBuilder,
		eventController:  eventController,
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), policyWorkQueueName),
	}

	policyInformer.GetInfomer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.createPolicyHandler,
		UpdateFunc: controller.updatePolicyHandler,
		DeleteFunc: controller.deletePolicyHandler,
	})
	return controller
}

func (pc *PolicyController) createPolicyHandler(resource interface{}) {
	pc.enqueuePolicy(resource)
}

func (pc *PolicyController) updatePolicyHandler(oldResource, newResource interface{}) {
	newPolicy := newResource.(*types.Policy)
	oldPolicy := oldResource.(*types.Policy)
	if newPolicy.ResourceVersion == oldPolicy.ResourceVersion {
		return
	}
	pc.enqueuePolicy(newResource)
}

func (pc *PolicyController) deletePolicyHandler(resource interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = resource.(metav1.Object); !ok {
		glog.Error("error decoding object, invalid type")
		return
	}
	glog.Infof("policy deleted: %s", object.GetName())
}

func (pc *PolicyController) enqueuePolicy(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		glog.Error(err)
		return
	}
	pc.queue.Add(key)
}

// Run is main controller thread
func (pc *PolicyController) Run(stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()

	if ok := cache.WaitForCacheSync(stopCh, pc.policySynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	for i := 0; i < policyControllerWorkerCount; i++ {
		go wait.Until(pc.runWorker, time.Second, stopCh)
	}
	glog.Info("started policy controller workers")

	return nil
}

//Stop to perform actions when controller is stopped
func (pc *PolicyController) Stop() {
	defer pc.queue.ShutDown()
	glog.Info("shutting down policy controller workers")
}
func (pc *PolicyController) runWorker() {
	for pc.processNextWorkItem() {
	}
}

func (pc *PolicyController) processNextWorkItem() bool {
	obj, shutdown := pc.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer pc.queue.Done(obj)
		err := pc.syncHandler(obj)
		pc.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (pc *PolicyController) handleErr(err error, key interface{}) {
	if err == nil {
		pc.queue.Forget(key)
		return
	}
	// This controller retries if something goes wrong. After that, it stops trying.
	if pc.queue.NumRequeues(key) < policyWorkQueueRetryLimit {
		glog.Warningf("Error syncing events %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		pc.queue.AddRateLimited(key)
		return
	}
	pc.queue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key %q out of the queue: %v", key, err)
}

func (pc *PolicyController) syncHandler(obj interface{}) error {
	var key string
	var ok bool
	if key, ok = obj.(string); !ok {
		return fmt.Errorf("expected string in workqueue but got %#v", obj)
	}
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.Errorf("invalid policy key: %s", key)
		return nil
	}

	// Get Policy resource with namespace/name
	policy, err := pc.policyLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("policy '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}
	// process policy on existing resource
	// get the violations and pass to violation Builder
	// get the events and pass to event Builder
	//TODO: processPolicy
	glog.Infof("process policy %s on existing resources", policy.GetName())
	policyInfos := engine.ProcessExisting(pc.client, policy)
	events, violations := createEventsAndViolations(pc.eventController, policyInfos)
	pc.eventController.Add(events...)
	err = pc.violationBuilder.Add(violations...)
	if err != nil {
		glog.Error(err)
	}
	return nil
}

func createEventsAndViolations(eventController event.Generator, policyInfos []*info.PolicyInfo) ([]*event.Info, []*violation.Info) {
	events := []*event.Info{}
	violations := []*violation.Info{}
	// Create events from the policyInfo
	for _, policyInfo := range policyInfos {
		fruleNames := []string{}
		sruleNames := []string{}

		for _, rule := range policyInfo.Rules {
			if !rule.IsSuccessful() {
				e := &event.Info{}
				fruleNames = append(fruleNames, rule.Name)
				switch rule.RuleType {
				case info.Mutation, info.Validation, info.Generation:
					// Events
					e = event.NewEvent(policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyViolation, event.FProcessRule, rule.Name, policyInfo.Name)
				default:
					glog.Info("Unsupported Rule type")
				}
				events = append(events, e)
			} else {
				sruleNames = append(sruleNames, rule.Name)
			}
		}

		if !policyInfo.IsSuccessful() {
			// Event
			// list of failed rules : ruleNames
			e := event.NewEvent("Policy", "", policyInfo.Name, event.PolicyViolation, event.FResourcePolcy, policyInfo.RNamespace+"/"+policyInfo.RName, strings.Join(fruleNames, ";"))
			events = append(events, e)
			// Violation
			v := violation.NewViolationFromEvent(e, policyInfo.Name, policyInfo.RKind, policyInfo.RName, policyInfo.RNamespace)
			violations = append(violations, v)
		}
		// else {
		// 	// Policy was processed succesfully
		// 	e := event.NewEvent("Policy", "", policyInfo.Name, event.PolicyApplied, event.SPolicyApply, policyInfo.Name)
		// 	events = append(events, e)
		// 	// Policy applied succesfully on resource
		// 	e = event.NewEvent(policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyApplied, event.SRuleApply, strings.Join(sruleNames, ";"), policyInfo.RName)
		// 	events = append(events, e)
		// }
	}
	return events, violations
}
