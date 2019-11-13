package policyviolation

import (
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	kyvernoclient "github.com/nirmata/kyverno/pkg/client/clientset/versioned"
	kyvernov1alpha1 "github.com/nirmata/kyverno/pkg/client/clientset/versioned/typed/kyverno/v1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	dclient "github.com/nirmata/kyverno/pkg/dclient"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

const workQueueName = "policy-violation-controller"
const workQueueRetryLimit = 3

//Generator creates PV
type Generator struct {
	dclient     *dclient.Client
	pvInterface kyvernov1alpha1.KyvernoV1Interface
	pvLister    kyvernolister.ClusterPolicyViolationLister
	nspvLister  kyvernolister.NamespacedPolicyViolationLister
	queue       workqueue.RateLimitingInterface
	dataStore   *dataStore
}

func NewDataStore() *dataStore {
	ds := dataStore{
		data: make(map[string]Info),
	}
	return &ds
}

type dataStore struct {
	data map[string]Info
	mu   sync.RWMutex
}

func (ds *dataStore) add(keyHash string, info Info) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	// queue the key hash
	ds.data[keyHash] = info
}

func (ds *dataStore) lookup(keyHash string) Info {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.data[keyHash]
}

func (ds *dataStore) delete(keyHash string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	delete(ds.data, keyHash)
}

//Info is a request to create PV
type Info struct {
	Blocked    bool
	PolicyName string
	Resource   unstructured.Unstructured
	Rules      []kyverno.ViolatedRule
}

func (i Info) toKey() string {
	keys := []string{
		strconv.FormatBool(i.Blocked),
		i.PolicyName,
		i.Resource.GetKind(),
		i.Resource.GetNamespace(),
		i.Resource.GetName(),
		strconv.Itoa(len(i.Rules)),
	}
	return strings.Join(keys, "/")
}

// make the struct hashable

//GeneratorInterface provides API to create PVs
type GeneratorInterface interface {
	Add(infos ...Info)
}

// NewPVGenerator returns a new instance of policy violation generator
func NewPVGenerator(client *kyvernoclient.Clientset, dclient *client.Client,
	pvLister kyvernolister.ClusterPolicyViolationLister,
	nspvLister kyvernolister.NamespacedPolicyViolationLister) *Generator {
	gen := Generator{
		pvInterface: client.KyvernoV1(),
		dclient:     dclient,
		pvLister:    pvLister,
		nspvLister:  nspvLister,
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), workQueueName),
		dataStore:   NewDataStore(),
	}
	return &gen
}

func (gen *Generator) enqueue(info Info) {
	// add to data map
	keyHash := info.toKey()
	// add to
	// queue the key hash
	gen.dataStore.add(keyHash, info)
	gen.queue.Add(keyHash)
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
	glog.Info("Start policy violation generator")
	defer glog.Info("Shutting down policy violation generator")

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
		return
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
	// remove from data store
	if keyHash, ok := key.(string); ok {
		gen.dataStore.delete(keyHash)
	}

	glog.Warningf("Dropping the key out of the queue: %v", err)
}

func (gen *Generator) processNextWorkitem() bool {
	obj, shutdown := gen.queue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer gen.queue.Done(obj)
		var keyHash string
		var ok bool
		if keyHash, ok = obj.(string); !ok {
			gen.queue.Forget(obj)
			glog.Warningf("Expecting type string but got %v\n", obj)
			return nil
		}
		// lookup data store
		info := gen.dataStore.lookup(keyHash)
		if reflect.DeepEqual(info, Info{}) {
			// empty key
			gen.queue.Forget(obj)
			glog.Warningf("Got empty key %v\n", obj)
			return nil
		}
		err := gen.syncHandler(info)
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
	glog.V(4).Infof("recieved info:%v", info)
	// cluster policy violations
	if info.Resource.GetNamespace() == "" {
		var pvs []kyverno.ClusterPolicyViolation
		if !info.Blocked {
			pvs = append(pvs, buildPV(info))
		} else {
			// blocked
			// get owners
			pvs = buildPVWithOwners(gen.dclient, info)
		}
		// create policy violation
		if err := createPVS(gen.dclient, pvs, gen.pvLister, gen.pvInterface); err != nil {
			return err
		}

		glog.V(3).Infof("Created cluster policy violation policy=%s, resource=%s/%s/%s",
			info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName())
		return nil
	}

	// namespaced policy violations
	var pvs []kyverno.NamespacedPolicyViolation
	if !info.Blocked {
		pvs = append(pvs, buildNamespacedPV(info))
	} else {
		pvs = buildNamespacedPVWithOwner(gen.dclient, info)
	}

	if err := createNamespacedPV(gen.dclient, gen.nspvLister, gen.pvInterface, pvs); err != nil {
		return err
	}

	glog.V(3).Infof("Created namespaced policy violation policy=%s, resource=%s/%s/%s",
		info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName())
	return nil
}

func createPVS(dclient *client.Client, pvs []kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface kyvernov1alpha1.KyvernoV1Interface) error {
	for _, pv := range pvs {
		if err := createPVNew(dclient, pv, pvLister, pvInterface); err != nil {
			return err
		}
	}
	return nil
}

func createPVNew(dclient *client.Client, pv kyverno.ClusterPolicyViolation, pvLister kyvernolister.ClusterPolicyViolationLister, pvInterface kyvernov1alpha1.KyvernoV1Interface) error {
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
		err := retryGetResource(dclient, pv.Spec.ResourceSpec)
		if err != nil {
			return err
		}

		_, err = pvInterface.ClusterPolicyViolations().Create(&pv)
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

	_, err = pvInterface.ClusterPolicyViolations().Update(&pv)
	if err != nil {
		glog.Error(err)
		return err
	}
	glog.Infof("policy violation updated for resource %v", pv.Spec.ResourceSpec)
	return nil
}

func getExistingPVIfAny(pvLister kyvernolister.ClusterPolicyViolationLister, currpv kyverno.ClusterPolicyViolation) (*kyverno.ClusterPolicyViolation, error) {
	pvs, err := pvLister.List(labels.Everything())
	if err != nil {
		glog.Errorf("unable to list policy violations : %v", err)
		return nil, err
	}

	for _, pv := range pvs {
		// find a policy on same resource and policy combination
		if pv.Spec.Policy == currpv.Spec.Policy &&
			pv.Spec.ResourceSpec.Kind == currpv.Spec.ResourceSpec.Kind &&
			pv.Spec.ResourceSpec.Namespace == currpv.Spec.ResourceSpec.Namespace &&
			pv.Spec.ResourceSpec.Name == currpv.Spec.ResourceSpec.Name {
			return pv, nil
		}
	}
	return nil, nil
}

// build PV without owners
func buildPV(info Info) kyverno.ClusterPolicyViolation {
	pv := buildPVObj(info.PolicyName, kyverno.ResourceSpec{
		Kind:      info.Resource.GetKind(),
		Namespace: info.Resource.GetNamespace(),
		Name:      info.Resource.GetName(),
	}, info.Rules,
	)
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

	labelMap := map[string]string{
		"policy":   policyName,
		"resource": resourceSpec.ToKey(),
	}
	pv.SetLabels(labelMap)
	pv.SetGenerateName("pv-")
	return pv
}

// build PV with owners
func buildPVWithOwners(dclient *client.Client, info Info) []kyverno.ClusterPolicyViolation {
	var pvs []kyverno.ClusterPolicyViolation
	// as its blocked resource, the violation is created on owner
	ownerMap := map[kyverno.ResourceSpec]interface{}{}
	GetOwner(dclient, ownerMap, info.Resource)

	// standaloneresource, set pvResourceSpec with resource itself
	if len(ownerMap) == 0 {
		pvResourceSpec := kyverno.ResourceSpec{
			Namespace: info.Resource.GetNamespace(),
			Kind:      info.Resource.GetKind(),
			Name:      info.Resource.GetName(),
		}
		return append(pvs, buildPVObj(info.PolicyName, pvResourceSpec, info.Rules))
	}

	// Generate owner on all owners
	for owner := range ownerMap {
		pv := buildPVObj(info.PolicyName, owner, info.Rules)
		pvs = append(pvs, pv)
	}
	return pvs
}

// GetOwner of a resource by iterating over ownerReferences
func GetOwner(dclient *client.Client, ownerMap map[kyverno.ResourceSpec]interface{}, resource unstructured.Unstructured) {
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
		GetOwner(dclient, ownerMap, *owner)
	}
}
