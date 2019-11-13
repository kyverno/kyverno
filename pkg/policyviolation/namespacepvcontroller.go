package policyviolation

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	backoff "github.com/cenkalti/backoff"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
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

var nspvcontrollerKind = kyverno.SchemeGroupVersion.WithKind("NamespacedPolicyViolation")

// PolicyViolationController manages the policy violation resource
// - sync the lastupdate time
// - check if the resource is active
type NamespacedPolicyViolationController struct {
	client                 *client.Client
	kyvernoClient          *kyvernoclient.Clientset
	eventRecorder          record.EventRecorder
	syncHandler            func(pKey string) error
	enqueuePolicyViolation func(policy *kyverno.NamespacedPolicyViolation)
	// Policys that need to be synced
	queue workqueue.RateLimitingInterface
	// nspvLister can list/get policy violation from the shared informer's store
	nspvLister kyvernolister.NamespacedPolicyViolationLister
	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// pvListerSynced retrns true if the Policy store has been synced at least once
	nspvListerSynced cache.InformerSynced
	//pvControl is used for updating status/cleanup policy violation
	pvControl NamespacedPVControlInterface
}

//NewPolicyViolationController creates a new NewPolicyViolationController
func NewNamespacedPolicyViolationController(client *client.Client, kyvernoClient *kyvernoclient.Clientset, pInformer kyvernoinformer.ClusterPolicyInformer, pvInformer kyvernoinformer.NamespacedPolicyViolationInformer) (*NamespacedPolicyViolationController, error) {
	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		return nil, err
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pvc := NamespacedPolicyViolationController{
		kyvernoClient: kyvernoClient,
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "ns_policyviolation_controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ns_policyviolation"),
	}
	pvc.pvControl = RealNamespacedPVControl{Client: kyvernoClient, Recorder: pvc.eventRecorder}
	pvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pvc.addPolicyViolation,
		UpdateFunc: pvc.updatePolicyViolation,
		DeleteFunc: pvc.deletePolicyViolation,
	})

	pvc.enqueuePolicyViolation = pvc.enqueue
	pvc.syncHandler = pvc.syncPolicyViolation

	pvc.pLister = pInformer.Lister()
	pvc.nspvLister = pvInformer.Lister()
	pvc.pListerSynced = pInformer.Informer().HasSynced
	pvc.nspvListerSynced = pvInformer.Informer().HasSynced

	return &pvc, nil
}

func (pvc *NamespacedPolicyViolationController) addPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.NamespacedPolicyViolation)
	glog.V(4).Infof("Adding Namespaced Policy Violation %s", pv.Name)
	pvc.enqueuePolicyViolation(pv)
}

func (pvc *NamespacedPolicyViolationController) updatePolicyViolation(old, cur interface{}) {
	oldPv := old.(*kyverno.NamespacedPolicyViolation)
	curPv := cur.(*kyverno.NamespacedPolicyViolation)
	glog.V(4).Infof("Updating Namespaced Policy Violation %s", oldPv.Name)
	if err := pvc.syncLastUpdateTimeStatus(curPv, oldPv); err != nil {
		glog.Errorf("Failed to update lastUpdateTime in NamespacedPolicyViolation %s status: %v", curPv.Name, err)
	}
	pvc.enqueuePolicyViolation(curPv)
}

func (pvc *NamespacedPolicyViolationController) deletePolicyViolation(obj interface{}) {
	pv, ok := obj.(*kyverno.NamespacedPolicyViolation)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.NamespacedPolicyViolation)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a NamespacedPolicyViolation %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting NamespacedPolicyViolation %s", pv.Name)
	pvc.enqueuePolicyViolation(pv)
}

func (pvc *NamespacedPolicyViolationController) enqueue(policyViolation *kyverno.NamespacedPolicyViolation) {
	key, err := cache.MetaNamespaceKeyFunc(policyViolation)
	if err != nil {
		glog.Error(err)
		return
	}
	pvc.queue.Add(key)
}

// Run begins watching and syncing.
func (pvc *NamespacedPolicyViolationController) Run(workers int, stopCh <-chan struct{}) {

	defer utilruntime.HandleCrash()
	defer pvc.queue.ShutDown()

	glog.Info("Starting Namespaced policyviolation controller")
	defer glog.Info("Shutting down Namespaced policyviolation controller")

	if !cache.WaitForCacheSync(stopCh, pvc.pListerSynced, pvc.nspvListerSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(pvc.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pvc *NamespacedPolicyViolationController) worker() {
	for pvc.processNextWorkItem() {
	}
}

func (pvc *NamespacedPolicyViolationController) processNextWorkItem() bool {
	key, quit := pvc.queue.Get()
	if quit {
		return false
	}
	defer pvc.queue.Done(key)

	err := pvc.syncHandler(key.(string))
	pvc.handleErr(err, key)

	return true
}

func (pvc *NamespacedPolicyViolationController) handleErr(err error, key interface{}) {
	if err == nil {
		pvc.queue.Forget(key)
		return
	}

	if pvc.queue.NumRequeues(key) < maxRetries {
		glog.V(2).Infof("Error syncing PolicyViolation %v: %v", key, err)
		pvc.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(2).Infof("Dropping policyviolation %q out of the queue: %v", key, err)
	pvc.queue.Forget(key)
}

func (pvc *NamespacedPolicyViolationController) syncPolicyViolation(key string) error {
	startTime := time.Now()
	glog.V(4).Infof("Started syncing policy violation %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing policy violation %q (%v)", key, time.Since(startTime))
	}()

	// tags: NAMESPACE/NAME
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("error getting namespaced policy violation key %v", key)
	}

	policyViolation, err := pvc.nspvLister.NamespacedPolicyViolations(ns).Get(name)
	if errors.IsNotFound(err) {
		glog.V(2).Infof("PolicyViolation %v has been deleted", key)
		return nil
	}

	if err != nil {
		return err
	}

	// Deep-copy otherwise we are mutating our cache.
	// TODO: Deep-copy only when needed.
	pv := policyViolation.DeepCopy()
	// TODO: Update Status to update ObserverdGeneration
	// TODO: check if the policy violation refers to a resource thats active ? // done by policy controller
	// TODO: remove the PV, if the corresponding policy is not present
	// TODO: additional check on deleted webhook for a resource, to delete a policy violation it has a policy violation
	// list the resource with label selectors, but this can be expensive for each delete request of a resource
	if err := pvc.syncActiveResource(pv); err != nil {
		glog.V(4).Infof("not syncing policy violation status")
		return err
	}

	return pvc.syncStatusOnly(pv)
}

func (pvc *NamespacedPolicyViolationController) syncActiveResource(curPv *kyverno.NamespacedPolicyViolation) error {
	// check if the resource is active or not ?
	rspec := curPv.Spec.ResourceSpec
	// get resource
	_, err := pvc.client.GetResource(rspec.Kind, rspec.Namespace, rspec.Name)
	if errors.IsNotFound(err) {
		// TODO: does it help to retry?
		// resource is not found
		// remove the violation

		if err := pvc.pvControl.RemovePolicyViolation(curPv.Namespace, curPv.Name); err != nil {
			glog.Infof("unable to delete the policy violation %s: %v", curPv.Name, err)
			return err
		}
		glog.V(4).Infof("removing policy violation %s as the corresponding resource %s/%s/%s does not exist anymore", curPv.Name, rspec.Kind, rspec.Namespace, rspec.Name)
		return nil
	}
	if err != nil {
		glog.V(4).Infof("error while retrieved resource %s/%s/%s: %v", rspec.Kind, rspec.Namespace, rspec.Name, err)
		return err
	}

	// cleanup pv with dependant
	if err := pvc.syncBlockedResource(curPv); err != nil {
		return err
	}

	//TODO- if the policy is not present, remove the policy violation
	return nil
}

// syncBlockedResource remove inactive policy violation
// when rejected resource created in the cluster
func (pvc *NamespacedPolicyViolationController) syncBlockedResource(curPv *kyverno.NamespacedPolicyViolation) error {
	for _, violatedRule := range curPv.Spec.ViolatedRules {
		if reflect.DeepEqual(violatedRule.ManagedResource, kyverno.ManagedResourceSpec{}) {
			continue
		}

		// get resource
		blockedResource := violatedRule.ManagedResource
		resources, _ := pvc.client.ListResource(blockedResource.Kind, blockedResource.Namespace, nil)

		for _, resource := range resources.Items {
			glog.V(4).Infof("getting owners for %s/%s/%s\n", resource.GetKind(), resource.GetNamespace(), resource.GetName())
			owners := map[kyverno.ResourceSpec]interface{}{}
			GetOwner(pvc.client, owners, resource) // owner of resource matches violation resourceSpec
			// remove policy violation as the blocked request got created
			if _, ok := owners[curPv.Spec.ResourceSpec]; ok {
				// pod -> replicaset1; deploy -> replicaset2
				// if replicaset1 == replicaset2, the pod is
				// no longer an active child of deploy, skip removing pv
				if !validDependantForDeployment(pvc.client.GetAppsV1Interface(), curPv.Spec.ResourceSpec, resource) {
					glog.V(4).Infof("")
					continue
				}

				// resource created, remove policy violation
				if err := pvc.pvControl.RemovePolicyViolation(curPv.Namespace, curPv.Name); err != nil {
					glog.Infof("unable to delete the policy violation %s: %v", curPv.Name, err)
					return err
				}
				glog.V(4).Infof("removed policy violation %s as the blocked resource %s/%s successfully created, owner: %s",
					curPv.Name, blockedResource.Kind, blockedResource.Namespace, strings.ReplaceAll(curPv.Spec.ResourceSpec.ToKey(), ".", "/"))
			}
		}
	}
	return nil
}

//syncStatusOnly updates the policyviolation status subresource
// status:
func (pvc *NamespacedPolicyViolationController) syncStatusOnly(curPv *kyverno.NamespacedPolicyViolation) error {
	// newStatus := calculateStatus(pv)
	return nil
}

//TODO: think this through again
//syncLastUpdateTimeStatus updates the policyviolation lastUpdateTime if anything in ViolationSpec changed
// 		- lastUpdateTime : (time stamp when the policy violation changed)
func (pvc *NamespacedPolicyViolationController) syncLastUpdateTimeStatus(curPv *kyverno.NamespacedPolicyViolation, oldPv *kyverno.NamespacedPolicyViolation) error {
	// check if there is any change in policy violation information
	if !updatedNamespaced(curPv, oldPv) {
		return nil
	}
	// update the lastUpdateTime
	newPolicyViolation := curPv
	newPolicyViolation.Status = kyverno.PolicyViolationStatus{LastUpdateTime: metav1.Now()}

	return pvc.pvControl.UpdateStatusPolicyViolation(newPolicyViolation)
}

func updatedNamespaced(curPv *kyverno.NamespacedPolicyViolation, oldPv *kyverno.NamespacedPolicyViolation) bool {
	return !reflect.DeepEqual(curPv.Spec, oldPv.Spec)
	//TODO check if owner reference changed, then should we update the lastUpdateTime as well ?
}

type NamespacedPVControlInterface interface {
	UpdateStatusPolicyViolation(newPv *kyverno.NamespacedPolicyViolation) error
	RemovePolicyViolation(ns, name string) error
}

// RealNamespacedPVControl is the default implementation of NamespacedPVControlInterface.
type RealNamespacedPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//UpdateStatusPolicyViolation updates the status for policy violation
func (r RealNamespacedPVControl) UpdateStatusPolicyViolation(newPv *kyverno.NamespacedPolicyViolation) error {
	_, err := r.Client.KyvernoV1().NamespacedPolicyViolations(newPv.Namespace).UpdateStatus(newPv)
	return err
}

//RemovePolicyViolation removes the policy violation
func (r RealNamespacedPVControl) RemovePolicyViolation(ns, name string) error {
	return r.Client.KyvernoV1().NamespacedPolicyViolations(ns).Delete(name, &metav1.DeleteOptions{})
}

func retryGetResource(client *client.Client, rspec kyverno.ResourceSpec) error {
	var i int
	getResource := func() error {
		_, err := client.GetResource(rspec.Kind, rspec.Namespace, rspec.Name)
		glog.V(5).Infof("retry %v getting %s/%s/%s", i, rspec.Kind, rspec.Namespace, rspec.Name)
		i++
		return err
	}

	exbackoff := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         time.Second,
		MaxElapsedTime:      3 * time.Second,
		Clock:               backoff.SystemClock,
	}

	exbackoff.Reset()
	err := backoff.Retry(getResource, exbackoff)
	if err != nil {
		return err
	}

	return nil
}
