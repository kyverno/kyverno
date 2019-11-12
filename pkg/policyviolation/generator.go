package policyviolation

import (
	"fmt"
	"reflect"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	pvInterface "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Generator creates PV
type Generator struct {
	dclient     *dclient.Client
	pvInterface pvInterface.ClusterPolicyViolationInterface
	pvLister    kyvernolister.ClusterPolicyViolationLister
	queue       workqueue.RateLimitingInterface
}

//Info is a request to create PV
type Info struct {
	Blocked    bool
	PolicyName string
	Resource   unstructured.Unstructured
	Rules      []kyverno.ViolatedRule
}

// make the struct hashable

//GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

// NewPVGenerator returns a new instance of policy violation generator
func NewPVGenerator(client *kyvernoclient.Clientset,
	pvLister kyvernolister.ClusterPolicyViolationLister) *Generator {
	gen := Generator{
		pvInterface: client.KyvernoV1alpha1().ClusterPolicyViolations(),
		pvLister:    pvLister,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
	}
	return &gen
}

func (gen *Generator) enqueue(info Info) {
	key, err := cache.MetaNamespaceKeyFunc(info)
	if err != nil {
		glog.Error(err)
		return
	}
	gen.queue.Add(key)
}

//Add queues a policy violation create request
func (gen *Generator) Add(infos ...Info) {
	for _, info := range infos {
		gen.enqueue(info)
	}
}

// Run starts the workers
func (gen *Generator) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	glog.Info("Start policy violaion generator")
	defer glog.Info("Shutting down event generator")

	for i := 0; i < workers; i++ {
		go wait.Until(gen.runWorker, time.Second, stopCh)
	}
	<-stopCh
}

func (gen *Generator) runWorker() {
	for gen.processNextWorkitem() {
	}
}

func (gen *Generator) handleErr(err error, key interface{}) {
	if err == nil {
		gen.queue.Forget(key)
	}

	// retires requests if there is error
	if gen.queue.NumRequeues(key) < workQueueRetryLimit {
		glog.Warningf("Error syncing policy violation %v: %v", key, err)
		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		gen.queue.AddRateLimited(key)
		return
	}
	gen.queue.Forget(key)
	glog.Error(err)
	glog.Warningf("Dropping the key out of the queue: %v", err)
}

func (gen *Generator) processNextWorkitem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer gen.queue.Done(obj)
		var key Info
		var ok bool
		if key, ok = obj.(Info); !ok {
			gen.queue.Forget(obj)
			glog.Warningf("Expecting type info bt got %v\n", obj)
			return nil
		}
		err := gen.syncHandler(key)
		gen.handleErr(err, obj)
		return nil
	}(obj)
	if err != nil {
		glog.Error(err)
		return true
	}
	return true
}

func (gen *Generator) syncHandler(info Info) error {
	var pvs []kyverno.ClusterPolicyViolation
	if !info.Blocked {
		pvs = append(pvs, buildPV(info))
	} else {
		// blocked
		// get owners
		pvs = buildPVWithOwners(gen.dclient, info)
	}
	// create policy violation
	createPVS(pvs, gen.pvLister, gen.pvInterface)
	return nil
}

func createPVS(pvs []kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface pvInterface.ClusterPolicyViolationInterface) {
	for _, pv := range pvs {
		createPVNew(pv, pvLister, pvInterface)
	}
}

func createPVNew(pv kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface pvInterface.ClusterPolicyViolationInterface) error {
	var err error
	// PV already exists
	ePV, err := getExistingPVIfAny(pvLister, pv)
	if err != nil {
		glog.Error(err)
		return err
	}
	if ePV == nil {
		// Create a New PV
		glog.V(4).Infof("creating new policy violation for policy %s & resource %s/%s/%s", pv.Spec.Policy, pv.Spec.ResourceSpec.Kind, pv.Spec.ResourceSpec.Namespace, pv.Spec.ResourceSpec.Name)
		_, err = pvInterface.Create(&pv)
		if err != nil {
			glog.Error(err)
			return err
		}
		glog.Infof("policy violation created for resource %v", pv.Spec.ResourceSpec)
		return nil
	}
	// Update existing PV if there any changes
	if reflect.DeepEqual(pv.Spec, ePV.Spec) {
		glog.V(4).Infof("policy violation spec %v did not change so not updating it", pv.Spec)
		return nil
	}

	_, err = pvInterface.Update(&pv)
	if err != nil {
		glog.Error(err)
		return err
	}
	glog.Infof("policy violation updated for resource %v", pv.Spec.ResourceSpec)
	return nil
}

func getExistingPVIfAny(pvLister kyvernolister.ClusterPolicyViolationLister, pv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	labelMap := map[string]string{"policy": pv.Spec.Policy, "resource": pv.Spec.ResourceSpec.ToKey()}
	pvSelector, err := converLabelToSelector(labelMap)
	if err != nil {
		return nil, fmt.Errorf("failed to generate label sector of Policy name %s: %v", pv.Spec.Policy, err)
	}
	pvs, err := pvLister.List(pvSelector)
	if err != nil {
		glog.Errorf("unable to list policy violations with label selector %v: %v", pvSelector, err)
		return nil, err
	}

	if len(pvs) == 0 {
		glog.Infof("policy violation does not exist with labels %v", labelMap)
		return nil, nil
	}

	// There should be only one policy violation
	if len(pvs) > 1 {
		glog.Errorf("more than one policy violation exists  with labels %v", labelMap)
	}
	// return the first PV
	return pvs[0], nil
}

// build PV without owners
func buildPV(info Info) kyverno.ClusterPolicyViolation {
	pv := buildPVObj(info.PolicyName, kyverno.ResourceSpec{
		Kind:      info.Resource.GetKind(),
		Namespace: info.Resource.GetNamespace(),
		Name:      info.Resource.GetName(),
	}, info.Rules,
	)
	pv.SetGenerateName("pv-")
	return pv
}

// build PV object
func buildPVObj(policyName string, resourceSpec kyverno.ResourceSpec, rules []kyverno.ViolatedRule) kyverno.ClusterPolicyViolation {
	pv := kyverno.ClusterPolicyViolation{
		Spec: kyverno.PolicyViolationSpec{
			Policy:        policyName,
			ResourceSpec:  resourceSpec,
			ViolatedRules: rules,
		},
	}
	return pv
}

// build PV with owners
func buildPVWithOwners(dclient *client.Client, info Info) []kyverno.ClusterPolicyViolation {
	var pvs []kyverno.ClusterPolicyViolation
	// as its blocked resource, the violation is created on owner
	ownerMap := map[kyverno.ResourceSpec]interface{}{}
	getOwner(dclient, ownerMap, info.Resource)
	// Generate owner on all owners
	for owner := range ownerMap {
		pv := buildPVObj(info.PolicyName, owner, info.Rules)
		pvs = append(pvs, pv)
	}
	return pvs
}

// get owners of a resource by iterating over ownerReferences
func getOwner(dclient *client.Client, ownerMap map[kyverno.ResourceSpec]interface{}, resource unstructured.Unstructured) {
	var emptyInterface interface{}
	resourceSpec := kyverno.ResourceSpec{
		Kind:      resource.GetKind(),
		Namespace: resource.GetNamespace(),
		Name:      resource.GetName(),
	}
	if _, ok := ownerMap[resourceSpec]; ok {
		// owner seen before
		// breaking loop
		return
	}
	rOwners := resource.GetOwnerReferences()
	// if there are no resource owners then its top level resource
	if len(rOwners) == 0 {
		// add resource to map
		ownerMap[resourceSpec] = emptyInterface
		return
	}
	for _, rOwner := range rOwners {
		// lookup resource via client
		// owner has to be in same namespace
		owner, err := dclient.GetResource(rOwner.Kind, resource.GetNamespace(), rOwner.Name)
		if err != nil {
			glog.Errorf("Failed to get resource owner for %s/%s/%s, err: %v", rOwner.Kind, resource.GetNamespace(), rOwner.Name, err)
			// as we want to process other owners
			continue
		}
		getOwner(dclient, ownerMap, *owner)
	}
}
