package controller

import (
	"fmt"
	"reflect"
	"time"

	"github.com/nirmata/kyverno/pkg/annotations"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"

	"github.com/nirmata/kyverno/pkg/engine"

	"github.com/golang/glog"
	v1alpha1 "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
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
	client            *client.Client
	policyLister      lister.PolicyLister
	policySynced      cache.InformerSynced
	violationBuilder  violation.Generator
	eventController   event.Generator
	queue             workqueue.RateLimitingInterface
	filterK8Resources []utils.K8Resource
}

// NewPolicyController from cmd args
func NewPolicyController(client *client.Client,
	policyInformer sharedinformer.PolicyInformer,
	violationBuilder violation.Generator,
	eventController event.Generator,
	filterK8Resources string) *PolicyController {

	controller := &PolicyController{
		client:            client,
		policyLister:      policyInformer.GetLister(),
		policySynced:      policyInformer.GetInfomer().HasSynced,
		violationBuilder:  violationBuilder,
		eventController:   eventController,
		filterK8Resources: utils.ParseKinds(filterK8Resources),
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), policyWorkQueueName),
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
	newPolicy := newResource.(*v1alpha1.Policy)
	oldPolicy := oldResource.(*v1alpha1.Policy)
	newPolicy.Status = v1alpha1.Status{}
	oldPolicy.Status = v1alpha1.Status{}
	newPolicy.ResourceVersion = ""
	oldPolicy.ResourceVersion = ""
	if reflect.DeepEqual(newPolicy, oldPolicy) {
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
	cleanAnnotations(pc.client, resource, pc.filterK8Resources)
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
	pc.queue.ShutDown()
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
	glog.Warningf("Dropping the key out of the queue: %v", err)
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
	// Get Policy
	policy, err := pc.policyLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("policy '%s' in work queue no longer exists", key)
			return nil
		}
		return err
	}

	glog.Infof("process policy %s on existing resources", policy.GetName())
	// Process policy on existing resources
	policyInfos := engine.ProcessExisting(pc.client, policy, pc.filterK8Resources)

	events, violations := pc.createEventsAndViolations(policyInfos)
	// Events, Violations
	pc.eventController.Add(events...)
	err = pc.violationBuilder.Add(violations...)
	if err != nil {
		glog.Error(err)
	}

	// Annotations
	pc.createAnnotations(policyInfos)

	return nil
}

func (pc *PolicyController) createAnnotations(policyInfos []*info.PolicyInfo) {
	for _, pi := range policyInfos {
		//get resource
		obj, err := pc.client.GetResource(pi.RKind, pi.RNamespace, pi.RName)
		if err != nil {
			glog.Error(err)
			continue
		}
		// add annotation for policy application
		ann := obj.GetAnnotations()
		// if annotations are nil then create a map and patch
		// else
		// add the exact patch
		patch, err := annotations.PatchAnnotations(ann, pi, info.All)
		if patch == nil {
			/// nothing to patch
			return
		}
		_, err = pc.client.PatchResource(pi.RKind, pi.RNamespace, pi.RName, patch)
		if err != nil {
			glog.Error(err)
			continue
		}
	}
}

func (pc *PolicyController) createEventsAndViolations(policyInfos []*info.PolicyInfo) ([]*event.Info, []*violation.Info) {
	events := []*event.Info{}
	violations := []*violation.Info{}
	// Create events from the policyInfo
	for _, policyInfo := range policyInfos {
		frules := []v1alpha1.FailedRule{}
		sruleNames := []string{}

		for _, rule := range policyInfo.Rules {
			if !rule.IsSuccessful() {
				e := &event.Info{}
				frule := v1alpha1.FailedRule{Name: rule.Name}
				switch rule.RuleType {
				case info.Mutation, info.Validation, info.Generation:
					// Events
					e = event.NewEvent(policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyViolation, event.FProcessRule, rule.Name, policyInfo.Name)
					switch rule.RuleType {
					case info.Mutation:
						frule.Type = info.Mutation.String()
					case info.Validation:
						frule.Type = info.Validation.String()
					case info.Generation:
						frule.Type = info.Generation.String()
					}
					frule.Error = rule.GetErrorString()
				default:
					glog.Info("Unsupported Rule type")
				}
				frule.Error = rule.GetErrorString()
				frules = append(frules, frule)
				events = append(events, e)
			} else {
				sruleNames = append(sruleNames, rule.Name)
			}
		}

		if !policyInfo.IsSuccessful() {
			e := event.NewEvent("Policy", "", policyInfo.Name, event.PolicyViolation, event.FResourcePolcy, policyInfo.RNamespace+"/"+policyInfo.RName, concatFailedRules(frules))
			events = append(events, e)
			// Violation
			v := violation.BuldNewViolation(policyInfo.Name, policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyViolation.String(), policyInfo.GetFailedRules())
			violations = append(violations, v)
		} else {
			// clean up violations
			pc.violationBuilder.RemoveInactiveViolation(policyInfo.Name, policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, info.Mutation)
			pc.violationBuilder.RemoveInactiveViolation(policyInfo.Name, policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, info.Validation)
		}
	}
	return events, violations
}
