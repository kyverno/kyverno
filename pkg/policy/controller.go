package policy

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/config"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/event"
	"github.com/nirmata/kyverno/pkg/webhookconfig"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	webhookinformer "k8s.io/client-go/informers/admissionregistration/v1beta1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	webhooklister "k8s.io/client-go/listers/admissionregistration/v1beta1"
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
	pvLister kyvernolister.ClusterPolicyViolationLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// pvListerSynced returns true if the Policy store has been synced at least once
	pvListerSynced cache.InformerSynced
	// mutationwebhookLister can list/get mutatingwebhookconfigurations
	mutationwebhookLister webhooklister.MutatingWebhookConfigurationLister
	// WebhookRegistrationClient
	webhookRegistrationClient *webhookconfig.WebhookRegistrationClient
	// Resource manager, manages the mapping for already processed resource
	rm resourceManager
	// helpers to validate against current loaded configuration
	configHandler config.Interface
	// recieves stats and aggregates details
	statusAggregator *PolicyStatusAggregator
}

// NewPolicyController create a new PolicyController
func NewPolicyController(kyvernoClient *kyvernoclient.Clientset, client *client.Client, pInformer kyvernoinformer.ClusterPolicyInformer, pvInformer kyvernoinformer.ClusterPolicyViolationInformer,
	eventGen event.Interface, webhookInformer webhookinformer.MutatingWebhookConfigurationInformer, webhookRegistrationClient *webhookconfig.WebhookRegistrationClient,
	configHandler config.Interface) (*PolicyController, error) {
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
		// filterK8Resources:         utils.ParseKinds(filterK8Resources),
		configHandler: configHandler,
	}

	pc.pvControl = RealPVControl{Client: kyvernoClient, Recorder: pc.eventRecorder}

	pInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicy,
		UpdateFunc: pc.updatePolicy,
		DeleteFunc: pc.deletePolicy,
	})

	pvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pc.addPolicyViolation,
		UpdateFunc: pc.updatePolicyViolation,
		DeleteFunc: pc.deletePolicyViolation,
	})

	pc.enqueuePolicy = pc.enqueue
	pc.syncHandler = pc.syncPolicy

	pc.pLister = pInformer.Lister()
	pc.pvLister = pvInformer.Lister()
	pc.pListerSynced = pInformer.Informer().HasSynced
	pc.pvListerSynced = pInformer.Informer().HasSynced

	pc.mutationwebhookLister = webhookInformer.Lister()

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
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) updatePolicy(old, cur interface{}) {
	oldP := old.(*kyverno.ClusterPolicy)
	curP := cur.(*kyverno.ClusterPolicy)
	glog.V(4).Infof("Updating Policy %s", oldP.Name)
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
	pc.enqueuePolicy(p)
}

func (pc *PolicyController) addPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.ClusterPolicyViolation)

	if pv.DeletionTimestamp != nil {
		// On a restart of the controller manager, it's possible for an object to
		// show up in a state that is already pending deletion.
		pc.deletePolicyViolation(pv)
		return
	}

	// generate labels to match the policy from the spec, if not present
	if updatePolicyLabelIfNotDefined(pc.pvControl, pv) {
		return
	}

	// If it has a ControllerRef, that's all that matters.
	if controllerRef := metav1.GetControllerOf(pv); controllerRef != nil {
		p := pc.resolveControllerRef(controllerRef)
		if p == nil {
			return
		}
		glog.V(4).Infof("PolicyViolation %s added.", pv.Name)
		pc.enqueuePolicy(p)
		return
	}

	// Otherwise, it's an orphan. Get a list of all matching Policies and sync
	// them to see if anyone wants to adopt it.
	ps := pc.getPolicyForPolicyViolation(pv)
	if len(ps) == 0 {
		// there is no cluster policy for this violation, so we can delete this cluster policy violation
		glog.V(4).Infof("PolicyViolation %s does not belong to an active policy, will be cleanedup", pv.Name)
		if err := pc.pvControl.DeletePolicyViolation(pv.Name); err != nil {
			glog.Errorf("Failed to deleted policy violation %s: %v", pv.Name, err)
			return
		}
		glog.V(4).Infof("PolicyViolation %s deleted", pv.Name)
		return
	}
	glog.V(4).Infof("Orphan Policy Violation %s added.", pv.Name)
	for _, p := range ps {
		pc.enqueuePolicy(p)
	}
}

func (pc *PolicyController) updatePolicyViolation(old, cur interface{}) {
	curPV := cur.(*kyverno.ClusterPolicyViolation)
	oldPV := old.(*kyverno.ClusterPolicyViolation)
	if curPV.ResourceVersion == oldPV.ResourceVersion {
		// Periodic resync will send update events for all known Policy Violation.
		// Two different versions of the same replica set will always have different RVs.
		return
	}

	// generate labels to match the policy from the spec, if not present
	if updatePolicyLabelIfNotDefined(pc.pvControl, curPV) {
		return
	}

	curControllerRef := metav1.GetControllerOf(curPV)
	oldControllerRef := metav1.GetControllerOf(oldPV)
	controllerRefChanged := !reflect.DeepEqual(curControllerRef, oldControllerRef)
	if controllerRefChanged && oldControllerRef != nil {
		// The ControllerRef was changed. Sync the old controller, if any.
		if p := pc.resolveControllerRef(oldControllerRef); p != nil {
			pc.enqueuePolicy(p)
		}
	}
	// If it has a ControllerRef, that's all that matters.
	if curControllerRef != nil {
		p := pc.resolveControllerRef(curControllerRef)
		if p == nil {
			return
		}
		glog.V(4).Infof("PolicyViolation %s updated.", curPV.Name)
		pc.enqueuePolicy(p)
		return
	}

	// Otherwise, it's an orphan. If anything changed, sync matching controllers
	// to see if anyone wants to adopt it now.
	labelChanged := !reflect.DeepEqual(curPV.Labels, oldPV.Labels)
	if labelChanged || controllerRefChanged {
		ps := pc.getPolicyForPolicyViolation(curPV)
		if len(ps) == 0 {
			// there is no cluster policy for this violation, so we can delete this cluster policy violation
			glog.V(4).Infof("PolicyViolation %s does not belong to an active policy, will be cleanedup", curPV.Name)
			if err := pc.pvControl.DeletePolicyViolation(curPV.Name); err != nil {
				glog.Errorf("Failed to deleted policy violation %s: %v", curPV.Name, err)
				return
			}
			glog.V(4).Infof("PolicyViolation %s deleted", curPV.Name)
			return
		}
		glog.V(4).Infof("Orphan PolicyViolation %s updated", curPV.Name)
		for _, p := range ps {
			pc.enqueuePolicy(p)
		}
	}
}

// deletePolicyViolation enqueues the Policy that manages a PolicyViolation when
// the PolicyViolation is deleted. obj could be an *kyverno.CusterPolicyViolation, or
// a DeletionFinalStateUnknown marker item.

func (pc *PolicyController) deletePolicyViolation(obj interface{}) {
	pv, ok := obj.(*kyverno.ClusterPolicyViolation)
	// When a delete is dropped, the relist will notice a PolicyViolation in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value. Note that this value might be stale. If the PolicyViolation
	// changed labels the new Policy will not be woken up till the periodic resync.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.ClusterPolicyViolation)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
	}
	controllerRef := metav1.GetControllerOf(pv)
	if controllerRef == nil {
		// No controller should care about orphans being deleted.
		return
	}
	p := pc.resolveControllerRef(controllerRef)
	if p == nil {
		return
	}
	glog.V(4).Infof("PolicyViolation %s deleted", pv.Name)
	pc.enqueuePolicy(p)
}

// resolveControllerRef returns the controller referenced by a ControllerRef,
// or nil if the ControllerRef could not be resolved to a matching controller
// of the correct Kind.
func (pc *PolicyController) resolveControllerRef(controllerRef *metav1.OwnerReference) *kyverno.ClusterPolicy {
	// We can't look up by UID, so look up by Name and then verify UID.
	// Don't even try to look up by Name if it's the wrong Kind.
	if controllerRef.Kind != controllerRef.Kind {
		return nil
	}
	p, err := pc.pLister.Get(controllerRef.Name)
	if err != nil {
		return nil
	}
	if p.UID != controllerRef.UID {
		// The controller we found with this Name is not the same one that the
		// ControllerRef points to.
		return nil
	}
	return p
}

func (pc *PolicyController) getPolicyForPolicyViolation(pv *kyverno.ClusterPolicyViolation) []*kyverno.ClusterPolicy {
	policies, err := pc.pLister.GetPolicyForPolicyViolation(pv)
	if err != nil || len(policies) == 0 {
		return nil
	}
	// Because all PolicyViolations's belonging to a Policy should have a unique label key,
	// there should never be more than one Policy returned by the above method.
	// If that happens we should probably dynamically repair the situation by ultimately
	// trying to clean up one of the controllers, for now we just return the older one
	if len(policies) > 1 {
		// ControllerRef will ensure we don't do anything crazy, but more than one
		// item in this list nevertheless constitutes user error.
		glog.V(4).Infof("user error! more than one policy is selecting policy violation %s with labels: %#v, returning %s",
			pv.Name, pv.Labels, policies[0].Name)
	}
	return policies
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

	if !cache.WaitForCacheSync(stopCh, pc.pListerSynced, pc.pvListerSynced) {
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
		glog.V(4).Info(err)
		return err
	}

	if err := pc.createResourceMutatingWebhookConfigurationIfRequired(*policy); err != nil {
		glog.V(4).Infof("failed to create resource mutating webhook configurations, policies wont be applied on resources: %v", err)
		glog.Errorln(err)
	}

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	p := policy.DeepCopy()

	pvList, err := pc.getPolicyViolationsForPolicy(p)
	if err != nil {
		return err
	}
	// process policies on existing resources
	policyInfos := pc.processExistingResources(*p)
	// report errors
	pc.report(policyInfos)
	// fetch the policy again via the aggreagator to remain consistent
	// return pc.statusAggregator.UpdateViolationCount(p.Name, pvList)
	return pc.syncStatusOnly(p, pvList)
}

//syncStatusOnly updates the policy status subresource
// status:
// 		- violations : (count of the resources that violate this policy )
func (pc *PolicyController) syncStatusOnly(p *kyverno.ClusterPolicy, pvList []*kyverno.ClusterPolicyViolation) error {
	newStatus := pc.calculateStatus(p.Name, pvList)
	if reflect.DeepEqual(newStatus, p.Status) {
		// no update to status
		return nil
	}
	// update status
	newPolicy := p
	newPolicy.Status = newStatus
	_, err := pc.kyvernoClient.KyvernoV1alpha1().ClusterPolicies().UpdateStatus(newPolicy)
	return err
}

func (pc *PolicyController) calculateStatus(policyName string, pvList []*kyverno.ClusterPolicyViolation) kyverno.PolicyStatus {
	violationCount := len(pvList)
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

func (pc *PolicyController) getPolicyViolationsForPolicy(p *kyverno.ClusterPolicy) ([]*kyverno.ClusterPolicyViolation, error) {
	// List all PolicyViolation to find those we own but that no longer match our
	// selector. They will be orphaned by ClaimPolicyViolation().
	pvList, err := pc.pvLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	policyLabelmap := map[string]string{"policy": p.Name}
	//NOt using a field selector, as the match function will have to cash the runtime.object
	// to get the field, while it can get labels directly, saves the cast effort
	//spec.policyName!=default
	//	fs := fields.Set{"spec.name": name}.AsSelector().String()

	ls := &metav1.LabelSelector{}
	err = metav1.Convert_Map_string_To_string_To_v1_LabelSelector(&policyLabelmap, ls, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate label sector of Policy name %s: %v", p.Name, err)
	}
	policySelector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil, fmt.Errorf("Policy %s has invalid label selector: %v", p.Name, err)
	}

	canAdoptFunc := RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := pc.kyvernoClient.KyvernoV1alpha1().ClusterPolicies().Get(p.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if fresh.UID != p.UID {
			return nil, fmt.Errorf("original Policy %v is gone: got uid %v, wanted %v", p.Name, fresh.UID, p.UID)
		}
		return fresh, nil
	})

	cm := NewPolicyViolationControllerRefManager(pc.pvControl, p, policySelector, controllerKind, canAdoptFunc)

	return cm.claimPolicyViolations(pvList)
}

func (m *PolicyViolationControllerRefManager) claimPolicyViolations(sets []*kyverno.ClusterPolicyViolation) ([]*kyverno.ClusterPolicyViolation, error) {
	var claimed []*kyverno.ClusterPolicyViolation
	var errlist []error

	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.adoptPolicyViolation(obj.(*kyverno.ClusterPolicyViolation))
	}
	release := func(obj metav1.Object) error {
		return m.releasePolicyViolation(obj.(*kyverno.ClusterPolicyViolation))
	}

	for _, pv := range sets {
		ok, err := m.ClaimObject(pv, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, pv)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

func (m *PolicyViolationControllerRefManager) adoptPolicyViolation(pv *kyverno.ClusterPolicyViolation) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt PolicyViolation %v (%v): %v", pv.Name, pv.UID, err)
	}
	// Note that ValidateOwnerReferences() will reject this patch if another
	// OwnerReference exists with controller=true.
	//TODO Add JSON Patch Owner reference for resource
	//TODO Update owner refence for resource
	controllerFlag := true
	blockOwnerDeletionFlag := true
	pOwnerRef := metav1.OwnerReference{APIVersion: m.controllerKind.GroupVersion().String(),
		Kind:               m.controllerKind.Kind,
		Name:               m.Controller.GetName(),
		UID:                m.Controller.GetUID(),
		Controller:         &controllerFlag,
		BlockOwnerDeletion: &blockOwnerDeletionFlag,
	}
	addControllerPatch, err := createOwnerReferencePatch(pOwnerRef)
	if err != nil {
		glog.Errorf("failed to add owner reference %v for PolicyViolation %s: %v", pOwnerRef, pv.Name, err)
		return err
	}

	return m.pvControl.PatchPolicyViolation(pv.Name, addControllerPatch)
}

type patchOwnerReferenceValue struct {
	Op    string                  `json:"op"`
	Path  string                  `json:"path"`
	Value []metav1.OwnerReference `json:"value"`
}

func createOwnerReferencePatch(ownerRef metav1.OwnerReference) ([]byte, error) {
	payload := []patchOwnerReferenceValue{{
		Op:    "add",
		Path:  "/metadata/ownerReferences",
		Value: []metav1.OwnerReference{ownerRef},
	}}
	return json.Marshal(payload)
}

func removeOwnerReferencePatch(ownerRef metav1.OwnerReference) ([]byte, error) {
	payload := []patchOwnerReferenceValue{{
		Op:    "remove",
		Path:  "/metadata/ownerReferences",
		Value: []metav1.OwnerReference{ownerRef},
	}}
	return json.Marshal(payload)
}

func (m *PolicyViolationControllerRefManager) releasePolicyViolation(pv *kyverno.ClusterPolicyViolation) error {
	glog.V(2).Infof("patching PolicyViolation %s to remove its controllerRef to %s/%s:%s",
		pv.Name, m.controllerKind.GroupVersion(), m.controllerKind.Kind, m.Controller.GetName())
	//TODO JSON patch for owner reference for resources
	controllerFlag := true
	blockOwnerDeletionFlag := true
	pOwnerRef := metav1.OwnerReference{APIVersion: m.controllerKind.GroupVersion().String(),
		Kind:               m.controllerKind.Kind,
		Name:               m.Controller.GetName(),
		UID:                m.Controller.GetUID(),
		Controller:         &controllerFlag,
		BlockOwnerDeletion: &blockOwnerDeletionFlag,
	}

	removeControllerPatch, err := removeOwnerReferencePatch(pOwnerRef)
	if err != nil {
		glog.Errorf("failed to add owner reference %v for PolicyViolation %s: %v", pOwnerRef, pv.Name, err)
		return err
	}

	// deleteOwnerRefPatch := fmt.Sprintf(`{"metadata":{"ownerReferences":[{"$patch":"delete","uid":"%s"}],"uid":"%s"}}`, m.Controller.GetUID(), pv.UID)

	err = m.pvControl.PatchPolicyViolation(pv.Name, removeControllerPatch)
	if err != nil {
		if errors.IsNotFound(err) {
			// If the ReplicaSet no longer exists, ignore it.
			return nil
		}
		if errors.IsInvalid(err) {
			// Invalid error will be returned in two cases: 1. the ReplicaSet
			// has no owner reference, 2. the uid of the ReplicaSet doesn't
			// match, which means the ReplicaSet is deleted and then recreated.
			// In both cases, the error can be ignored.
			return nil
		}
	}
	return err
}

//PolicyViolationControllerRefManager manages adoption of policy violation by a policy
type PolicyViolationControllerRefManager struct {
	BaseControllerRefManager
	controllerKind schema.GroupVersionKind
	pvControl      PVControlInterface
}

//NewPolicyViolationControllerRefManager returns new PolicyViolationControllerRefManager
func NewPolicyViolationControllerRefManager(
	pvControl PVControlInterface,
	controller metav1.Object,
	selector labels.Selector,
	controllerKind schema.GroupVersionKind,
	canAdopt func() error,
) *PolicyViolationControllerRefManager {

	m := PolicyViolationControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   controller,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		controllerKind: controllerKind,
		pvControl:      pvControl,
	}
	return &m
}

//BaseControllerRefManager ...
type BaseControllerRefManager struct {
	Controller   metav1.Object
	Selector     labels.Selector
	canAdoptErr  error
	canAdoptOnce sync.Once
	CanAdoptFunc func() error
}

//CanAdopt ...
func (m *BaseControllerRefManager) CanAdopt() error {
	m.canAdoptOnce.Do(func() {
		if m.CanAdoptFunc != nil {
			m.canAdoptErr = m.CanAdoptFunc()
		}
	})
	return m.canAdoptErr
}

//ClaimObject ...
func (m *BaseControllerRefManager) ClaimObject(obj metav1.Object, match func(metav1.Object) bool, adopt, release func(metav1.Object) error) (bool, error) {
	controllerRef := metav1.GetControllerOf(obj)
	if controllerRef != nil {
		if controllerRef.UID != m.Controller.GetUID() {
			// Owned by someone else. Ignore
			return false, nil
		}
		if match(obj) {
			// We already own it and the selector matches.
			// Return true (successfully claimed) before checking deletion timestamp.
			// We're still allowed to claim things we already own while being deleted
			// because doing so requires taking no actions.
			return true, nil

		}
		// Owned by us but selector doesn't match.
		// Try to release, unless we're being deleted.
		if m.Controller.GetDeletionTimestamp() != nil {
			return false, nil
		}
		if err := release(obj); err != nil {
			// If the PolicyViolation no longer exists, ignore the error.
			if errors.IsNotFound(err) {
				return false, nil
			}
			// Either someone else released it, or there was a transient error.
			// The controller should requeue and try again if it's still stale.
			return false, err
		}
		// Successfully released.
		return false, nil
	}
	// It's an orphan.
	if m.Controller.GetDeletionTimestamp() != nil || !match(obj) {
		// Ignore if we're being deleted or selector doesn't match.
		return false, nil
	}
	if obj.GetDeletionTimestamp() != nil {
		// Ignore if the object is being deleted
		return false, nil
	}
	// Selector matches. Try to adopt.
	if err := adopt(obj); err != nil {
		// If the PolicyViolation no longer exists, ignore the error
		if errors.IsNotFound(err) {
			return false, nil
		}
		// Either someone else claimed it first, or there was a transient error.
		// The controller should requeue and try again if it's still orphaned.
		return false, err
	}
	// Successfully adopted.
	return true, nil

}

//PVControlInterface provides interface to  operate on policy violation resource
type PVControlInterface interface {
	PatchPolicyViolation(name string, data []byte) error
	DeletePolicyViolation(name string) error
}

// RealPVControl is the default implementation of PVControlInterface.
type RealPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//PatchPolicyViolation patches the policy violation with the provided JSON Patch
func (r RealPVControl) PatchPolicyViolation(name string, data []byte) error {
	_, err := r.Client.KyvernoV1alpha1().ClusterPolicyViolations().Patch(name, types.JSONPatchType, data)
	return err
}

//DeletePolicyViolation deletes the policy violation
func (r RealPVControl) DeletePolicyViolation(name string) error {
	return r.Client.KyvernoV1alpha1().ClusterPolicyViolations().Delete(name, &metav1.DeleteOptions{})
}

// RecheckDeletionTimestamp returns a CanAdopt() function to recheck deletion.
//
// The CanAdopt() function calls getObject() to fetch the latest value,
// and denies adoption attempts if that object has a non-nil DeletionTimestamp.
func RecheckDeletionTimestamp(getObject func() (metav1.Object, error)) func() error {
	return func() error {
		obj, err := getObject()
		if err != nil {
			return fmt.Errorf("can't recheck DeletionTimestamp: %v", err)
		}
		if obj.GetDeletionTimestamp() != nil {
			return fmt.Errorf("%v/%v has just been deleted at %v", obj.GetNamespace(), obj.GetName(), obj.GetDeletionTimestamp())
		}
		return nil
	}
}

type patchLabelValue struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value string `json:"value"`
}

type patchLabelMapValue struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

func createPolicyLabelPatch(policy string) ([]byte, error) {
	payload := []patchLabelValue{{
		Op:    "add",
		Path:  "/metadata/labels/policy",
		Value: policy,
	}}
	return json.Marshal(payload)
}

func createResourceLabelPatch(resource string) ([]byte, error) {
	payload := []patchLabelValue{{
		Op:    "add",
		Path:  "/metadata/labels/resource",
		Value: resource,
	}}
	return json.Marshal(payload)
}

func createLabelMapPatch(policy string, resource string) ([]byte, error) {
	payload := []patchLabelMapValue{{
		Op:    "add",
		Path:  "/metadata/labels",
		Value: map[string]string{"policy": policy, "resource": resource},
	}}
	return json.Marshal(payload)
}

//updatePolicyLabelIfNotDefined adds the label 'policy' to the PolicyViolation
// label is used here to lookup policyViolation and corresponding Policy
func updatePolicyLabelIfNotDefined(pvControl PVControlInterface, pv *kyverno.ClusterPolicyViolation) bool {
	updateLabel := func() bool {
		glog.V(4).Infof("adding label 'policy:%s' to PolicyViolation %s", pv.Spec.Policy, pv.Name)
		glog.V(4).Infof("adding label 'resource:%s' to PolicyViolation %s", pv.Spec.ResourceSpec.ToKey(), pv.Name)
		// add label based on the policy spec
		labels := pv.GetLabels()
		if pv.Spec.Policy == "" {
			glog.Error("policy not defined for violation")
			// should be cleaned up
			return false
		}
		if labels == nil {
			// create a patch to generate the labels map with policy label
			patch, err := createLabelMapPatch(pv.Spec.Policy, pv.Spec.ResourceSpec.ToKey())
			if err != nil {
				glog.Errorf("unable to init label map. %v", err)
				return false
			}
			if err := pvControl.PatchPolicyViolation(pv.Name, patch); err != nil {
				glog.Errorf("Unable to add 'policy' label to PolicyViolation %s: %v", pv.Name, err)
				return false
			}
			// update successful
			return true
		}
		// JSON Patch to add exact label
		policyLabelPatch, err := createPolicyLabelPatch(pv.Spec.Policy)
		if err != nil {
			glog.Errorf("failed to generate patch to add label 'policy': %v", err)
			return false
		}
		resourceLabelPatch, err := createResourceLabelPatch(pv.Spec.ResourceSpec.ToKey())
		if err != nil {
			glog.Errorf("failed to generate patch to add label 'resource': %v", err)
			return false
		}
		//join patches
		labelPatch := joinPatches(policyLabelPatch, resourceLabelPatch)
		if labelPatch == nil {
			glog.Errorf("failed to join patches : %v", err)
			return false
		}
		glog.V(4).Infof("patching policy violation %s with patch %s", pv.Name, string(labelPatch))
		if err := pvControl.PatchPolicyViolation(pv.Name, labelPatch); err != nil {
			glog.Errorf("Unable to add 'policy' label to PolicyViolation %s: %v", pv.Name, err)
			return false
		}
		// update successful
		return true
	}

	var policy string
	var ok bool
	// operate oncopy of resource
	curLabels := pv.GetLabels()
	if policy, ok = curLabels["policy"]; !ok {
		return updateLabel()
	}
	// TODO: would be benificial to add a check to verify if the policy in name and resource spec match
	if policy != pv.Spec.Policy {
		glog.Errorf("label 'policy:%s' and spec.policy %s dont match ", policy, pv.Spec.Policy)
		//TODO handle this case
		return updateLabel()
	}
	return false
}

func joinPatches(patches ...[]byte) []byte {
	var result []byte
	if patches == nil {
		//nothing tot join
		return result
	}
	result = append(result, []byte("[\n")...)
	for index, patch := range patches {
		result = append(result, patch...)
		if index != len(patches)-1 {
			result = append(result, []byte(",\n")...)
		}
	}
	result = append(result, []byte("\n]")...)
	return result
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
