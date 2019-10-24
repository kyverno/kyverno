package policyviolation

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	"github.com/nirmata/kyverno/pkg/client/clientset/versioned/scheme"
	kyvernoinformer "github.com/nirmata/kyverno/pkg/client/informers/externalversions/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
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

const (
	// maxRetries is the number of times a PolicyViolation will be retried before it is dropped out of the queue.
	// With the current rate-limiter in use (5ms*2^(maxRetries-1)) the following numbers represent the times
	// a deployment is going to be requeued:
	//
	// 5ms, 10ms, 20ms, 40ms, 80ms, 160ms, 320ms, 640ms, 1.3s, 2.6s, 5.1s, 10.2s, 20.4s, 41s, 82s
	maxRetries = 15
)

var controllerKind = kyverno.SchemeGroupVersion.WithKind("ClusterPolicyViolation")

// PolicyViolationController manages the policy violation resource
// - sync the lastupdate time
// - check if the resource is active
type PolicyViolationController struct {
	client                 *client.Client
	kyvernoClient          *kyvernoclient.Clientset
	eventRecorder          record.EventRecorder
	syncHandler            func(pKey string) error
	enqueuePolicyViolation func(policy *kyverno.ClusterPolicyViolation)
	// Policys that need to be synced
	queue workqueue.RateLimitingInterface
	// pvLister can list/get policy violation from the shared informer's store
	pvLister kyvernolister.ClusterPolicyViolationLister
	// pLister can list/get policy from the shared informer's store
	pLister kyvernolister.ClusterPolicyLister
	// pListerSynced returns true if the Policy store has been synced at least once
	pListerSynced cache.InformerSynced
	// pvListerSynced retrns true if the Policy store has been synced at least once
	pvListerSynced cache.InformerSynced
	//pvControl is used for updating status/cleanup policy violation
	pvControl PVControlInterface
}

//NewPolicyViolationController creates a new NewPolicyViolationController
func NewPolicyViolationController(client *client.Client, kyvernoClient *kyvernoclient.Clientset, pInformer kyvernoinformer.ClusterPolicyInformer, pvInformer kyvernoinformer.ClusterPolicyViolationInformer) (*PolicyViolationController, error) {
	// Event broad caster
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventInterface, err := client.GetEventsInterface()
	if err != nil {
		return nil, err
	}
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: eventInterface})

	pvc := PolicyViolationController{
		kyvernoClient: kyvernoClient,
		client:        client,
		eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "policyviolation_controller"}),
		queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "policyviolation"),
	}
	pvc.pvControl = RealPVControl{Client: kyvernoClient, Recorder: pvc.eventRecorder}
	pvInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    pvc.addPolicyViolation,
		UpdateFunc: pvc.updatePolicyViolation,
		DeleteFunc: pvc.deletePolicyViolation,
	})

	pvc.enqueuePolicyViolation = pvc.enqueue
	pvc.syncHandler = pvc.syncPolicyViolation

	pvc.pLister = pInformer.Lister()
	pvc.pvLister = pvInformer.Lister()
	pvc.pListerSynced = pInformer.Informer().HasSynced
	pvc.pvListerSynced = pvInformer.Informer().HasSynced

	return &pvc, nil
}

func (pvc *PolicyViolationController) addPolicyViolation(obj interface{}) {
	pv := obj.(*kyverno.ClusterPolicyViolation)
	glog.V(4).Infof("Adding PolicyViolation %s", pv.Name)
	pvc.enqueuePolicyViolation(pv)
}

func (pvc *PolicyViolationController) updatePolicyViolation(old, cur interface{}) {
	oldPv := old.(*kyverno.ClusterPolicyViolation)
	curPv := cur.(*kyverno.ClusterPolicyViolation)
	glog.V(4).Infof("Updating Policy Violation %s", oldPv.Name)
	if err := pvc.syncLastUpdateTimeStatus(curPv, oldPv); err != nil {
		glog.Errorf("Failed to update lastUpdateTime in PolicyViolation %s status: %v", curPv.Name, err)
	}
	pvc.enqueuePolicyViolation(curPv)
}

func (pvc *PolicyViolationController) deletePolicyViolation(obj interface{}) {
	pv, ok := obj.(*kyverno.ClusterPolicyViolation)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Info(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		pv, ok = tombstone.Obj.(*kyverno.ClusterPolicyViolation)
		if !ok {
			glog.Info(fmt.Errorf("Tombstone contained object that is not a PolicyViolation %#v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting PolicyViolation %s", pv.Name)
	pvc.enqueuePolicyViolation(pv)
}

func (pvc *PolicyViolationController) enqueue(policyViolation *kyverno.ClusterPolicyViolation) {
	key, err := cache.MetaNamespaceKeyFunc(policyViolation)
	if err != nil {
		glog.Error(err)
		return
	}
	pvc.queue.Add(key)
}

// Run begins watching and syncing.
func (pvc *PolicyViolationController) Run(workers int, stopCh <-chan struct{}) {

	defer utilruntime.HandleCrash()
	defer pvc.queue.ShutDown()

	glog.Info("Starting policyviolation controller")
	defer glog.Info("Shutting down policyviolation controller")

	if !cache.WaitForCacheSync(stopCh, pvc.pListerSynced, pvc.pvListerSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(pvc.worker, time.Second, stopCh)
	}
	<-stopCh
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (pvc *PolicyViolationController) worker() {
	for pvc.processNextWorkItem() {
	}
}

func (pvc *PolicyViolationController) processNextWorkItem() bool {
	key, quit := pvc.queue.Get()
	if quit {
		return false
	}
	defer pvc.queue.Done(key)

	err := pvc.syncHandler(key.(string))
	pvc.handleErr(err, key)

	return true
}

func (pvc *PolicyViolationController) handleErr(err error, key interface{}) {
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

func (pvc *PolicyViolationController) syncPolicyViolation(key string) error {
	startTime := time.Now()
	glog.V(4).Infof("Started syncing policy violation %q (%v)", key, startTime)
	defer func() {
		glog.V(4).Infof("Finished syncing policy violation %q (%v)", key, time.Since(startTime))
	}()
	policyViolation, err := pvc.pvLister.Get(key)
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

	if err := pvc.syncBlockedResource(pv); err != nil {
		glog.V(4).Infof("not syncing policy violation status")
		return err
	}

	return pvc.syncStatusOnly(pv)
}

// syncBlockedResource remove inactive policy violation
// when rejected resource created in the cluster
func (pvc *PolicyViolationController) syncBlockedResource(curPv *kyverno.ClusterPolicyViolation) error {
	for _, violatedRule := range curPv.Spec.ViolatedRules {
		if reflect.DeepEqual(violatedRule.Dependant, kyverno.Dependant{}) {
			continue
		}

		// get resource
		blockedResource := violatedRule.Dependant
		resources, _ := pvc.client.ListResource(blockedResource.Kind, blockedResource.Namespace, nil)

		for _, resource := range resources.Items {
			glog.V(4).Infof("getting owners for %s/%s/%s\n", resource.GetKind(), resource.GetNamespace(), resource.GetName())
			owners := getOwners(pvc.client, resource)
			// owner of resource matches violation resourceSpec
			// remove policy violation as the blocked request got created
			if containsOwner(owners, curPv) {
				// pod -> replicaset1; deploy -> replicaset2
				// if replicaset1 == replicaset2, the pod is
				// no longer an active child of deploy, skip removing pv
				if !validDependantForDeployment(pvc.client.GetAppsV1Interface(), *curPv, resource) {
					glog.V(4).Infof("")
					continue
				}

				// resource created, remove policy violation
				if err := pvc.pvControl.RemovePolicyViolation(curPv.Name); err != nil {
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

func (pvc *PolicyViolationController) syncActiveResource(curPv *kyverno.ClusterPolicyViolation) error {
	// check if the resource is active or not ?
	rspec := curPv.Spec.ResourceSpec
	// get resource
	_, err := pvc.client.GetResource(rspec.Kind, rspec.Namespace, rspec.Name)
	if errors.IsNotFound(err) {
		// TODO: does it help to retry?
		// resource is not found
		// remove the violation

		if err := pvc.pvControl.RemovePolicyViolation(curPv.Name); err != nil {
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
	//TODO- if the policy is not present, remove the policy violation
	return nil
}

//syncStatusOnly updates the policyviolation status subresource
// status:
func (pvc *PolicyViolationController) syncStatusOnly(curPv *kyverno.ClusterPolicyViolation) error {
	// newStatus := calculateStatus(pv)
	return nil
}

//TODO: think this through again
//syncLastUpdateTimeStatus updates the policyviolation lastUpdateTime if anything in ViolationSpec changed
// 		- lastUpdateTime : (time stamp when the policy violation changed)
func (pvc *PolicyViolationController) syncLastUpdateTimeStatus(curPv *kyverno.ClusterPolicyViolation, oldPv *kyverno.ClusterPolicyViolation) error {
	// check if there is any change in policy violation information
	if !updated(curPv, oldPv) {
		return nil
	}
	// update the lastUpdateTime
	newPolicyViolation := curPv
	newPolicyViolation.Status = kyverno.PolicyViolationStatus{LastUpdateTime: metav1.Now()}

	return pvc.pvControl.UpdateStatusPolicyViolation(newPolicyViolation)
}

func updated(curPv *kyverno.ClusterPolicyViolation, oldPv *kyverno.ClusterPolicyViolation) bool {
	return !reflect.DeepEqual(curPv.Spec, oldPv.Spec)
	//TODO check if owner reference changed, then should we update the lastUpdateTime as well ?
}

type PVControlInterface interface {
	UpdateStatusPolicyViolation(newPv *kyverno.ClusterPolicyViolation) error
	RemovePolicyViolation(name string) error
}

// RealPVControl is the default implementation of PVControlInterface.
type RealPVControl struct {
	Client   kyvernoclient.Interface
	Recorder record.EventRecorder
}

//UpdateStatusPolicyViolation updates the status for policy violation
func (r RealPVControl) UpdateStatusPolicyViolation(newPv *kyverno.ClusterPolicyViolation) error {
	_, err := r.Client.KyvernoV1alpha1().ClusterPolicyViolations().UpdateStatus(newPv)
	return err
}

//RemovePolicyViolation removes the policy violation
func (r RealPVControl) RemovePolicyViolation(name string) error {
	return r.Client.KyvernoV1alpha1().ClusterPolicyViolations().Delete(name, &metav1.DeleteOptions{})
}
