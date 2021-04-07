package policystatus

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
	kyvernolister "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	utils "github.com/kyverno/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

// Policy status implementation works in the following way,
// Currently policy status maintains a cache of the status of each policy.
// Every x unit of time the status of policy is updated using
//the data from the cache.
//The sync exposes a listener which accepts a statusUpdater
//interface which dictates how the status should be updated.
//The status is updated by a worker that receives the interface
//on a channel.
//The worker then updates the current status using the methods
//exposed by the interface.
//Current implementation is designed to be thread safe with optimized
//locking for each policy.

// statusUpdater defines a type to have a method which
// updates the given status
type statusUpdater interface {
	PolicyName() string
	UpdateStatus(status v1.PolicyStatus) v1.PolicyStatus
}

// Listener is a channel of statusUpdater instances
type Listener chan statusUpdater

// Update queues an status update request
func (l Listener) Update(s statusUpdater) {
	l <- s
}

// Sync is the object which is used to initialize
//the policyStatus sync, can be considered the parent object
//since it contains access to all the persistent data present
//in this package.
type Sync struct {
	cache    *cache
	Listener Listener
	client   *versioned.Clientset
	lister   kyvernolister.ClusterPolicyLister
	nsLister kyvernolister.PolicyLister
	log      logr.Logger
}

type cache struct {
	dataMu     sync.RWMutex
	data       map[string]v1.PolicyStatus
	keyToMutex *keyToMutex
}

// NewSync creates a new Sync instance
func NewSync(c *versioned.Clientset, lister kyvernolister.ClusterPolicyLister, nsLister kyvernolister.PolicyLister) *Sync {
	return &Sync{
		cache: &cache{
			dataMu:     sync.RWMutex{},
			data:       make(map[string]v1.PolicyStatus),
			keyToMutex: newKeyToMutex(),
		},
		client:   c,
		lister:   lister,
		nsLister: nsLister,
		Listener: make(chan statusUpdater, 20),
		log:      log.Log.WithName("PolicyStatus"),
	}
}

// Run starts workers and periodically flushes the cached status
func (s *Sync) Run(workers int, stopCh <-chan struct{}) {
	for i := 0; i < workers; i++ {
		go s.updateStatusCache(stopCh)
	}

	// sync the status to the existing policy every minute
	wait.Until(s.writePolicyStatus, time.Minute, stopCh)
	<-stopCh
}

// updateStatusCache is a worker which adds the current status
// to the cache, using the statusUpdater interface
func (s *Sync) updateStatusCache(stopCh <-chan struct{}) {
	for {
		select {
		case statusUpdater := <-s.Listener:
			name := statusUpdater.PolicyName()
			s.log.V(4).Info("received policy status update request", "policy", name)

			s.cache.keyToMutex.Get(name).Lock()

			s.cache.dataMu.RLock()
			status, exist := s.cache.data[statusUpdater.PolicyName()]
			s.cache.dataMu.RUnlock()
			if !exist {
				policy, _ := s.lister.Get(statusUpdater.PolicyName())
				if policy != nil {
					status = policy.Status
				}
			}

			updatedStatus := statusUpdater.UpdateStatus(status)

			s.cache.dataMu.Lock()
			s.cache.data[statusUpdater.PolicyName()] = updatedStatus
			s.cache.dataMu.Unlock()

			s.cache.keyToMutex.Get(statusUpdater.PolicyName()).Unlock()
			oldStatus, _ := json.Marshal(status)
			newStatus, _ := json.Marshal(updatedStatus)

			s.log.V(5).Info("updated policy status in the cache", "policy", statusUpdater.PolicyName(),
				"oldStatus", string(oldStatus), "newStatus", string(newStatus))

		case <-stopCh:
			return
		}
	}
}

// writePolicyStatus sends the update request to the APIServer
// syncs the status (from cache) to the policy
func (s *Sync) writePolicyStatus() {
	for key, status := range s.getCachedStatus() {
		s.log.V(4).Info("updating policy status", "policy", key)
		namespace, policyName := s.parseStatusKey(key)
		if namespace == "" {
			s.updateClusterPolicy(policyName, key, status)
		} else {
			s.updateNamespacedPolicyStatus(policyName, namespace, key, status)
		}
	}
}

func (s *Sync) parseStatusKey(key string) (string, string) {
	namespace := ""
	policyName := key

	index := strings.Index(key, "/")
	if index != -1 {
		namespace = key[:index]
		policyName = key[index+1:]
	}

	return namespace, policyName
}

func (s *Sync) updateClusterPolicy(policyName, key string, status v1.PolicyStatus) {
	defer s.deleteCachedStatus(key)

	policy, err := s.lister.Get(policyName)
	if err != nil {
		s.log.Error(err, "failed to update policy status", "policy", policyName)
		return
	}

	if reflect.DeepEqual(status, policy.Status) {
		return
	}
	if policy.Spec.Background == nil || policy.Spec.ValidationFailureAction == "" || checkAutoGenRules(policy.Spec.Rules) {
		policy.ObjectMeta.SetAnnotations(map[string]string{"kyverno.io/mutate-policy": "true"})
		_, err = s.client.KyvernoV1().ClusterPolicies().Update(context.TODO(), policy, metav1.UpdateOptions{})
		if err != nil {
			s.log.Error(err, "failed to update policy status", "policy", policyName)
		}
	}

	policy.Status = status
	_, err = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(context.TODO(), policy, metav1.UpdateOptions{})
	if err != nil {
		s.log.Error(err, "failed to update policy status", "policy", policyName)
	}
}

func (s *Sync) updateNamespacedPolicyStatus(policyName, namespace, key string, status v1.PolicyStatus) {
	defer s.deleteCachedStatus(key)

	policy, err := s.nsLister.Policies(namespace).Get(policyName)
	if err != nil {
		s.log.Error(err, "failed to update policy status", "policy", policyName)
		return
	}

	if reflect.DeepEqual(status, policy.Status) {
		return
	}

	if policy.Spec.Background == nil || policy.Spec.ValidationFailureAction == "" || checkAutoGenRules(policy.Spec.Rules) {
		policy.ObjectMeta.SetAnnotations(map[string]string{"kyverno.io/mutate-policy": "true"})
		_, err = s.client.KyvernoV1().Policies(namespace).UpdateStatus(context.TODO(), policy, metav1.UpdateOptions{})
		if err != nil {
			s.log.Error(err, "failed to update namespaced policy status", "policy", policyName)
		}
	}
	policy.Status = status
	_, err = s.client.KyvernoV1().Policies(namespace).UpdateStatus(context.TODO(), policy, metav1.UpdateOptions{})
	if err != nil {
		s.log.Error(err, "failed to update namespaced policy status", "policy", policyName)
	}
}

func (s *Sync) deleteCachedStatus(policyName string) {
	s.cache.dataMu.Lock()
	defer s.cache.dataMu.Unlock()

	delete(s.cache.data, policyName)
}

func (s *Sync) getCachedStatus() map[string]v1.PolicyStatus {
	s.cache.dataMu.Lock()
	defer s.cache.dataMu.Unlock()

	var nameToStatus = make(map[string]v1.PolicyStatus, len(s.cache.data))
	for k, v := range s.cache.data {
		nameToStatus[k] = v
	}

	return nameToStatus
}

func checkAutoGenRules(rule []v1.Rule) bool {
	if len(rule) != 0 && utils.ContainsString(rule[0].MatchResources.ResourceDescription.Kinds, "Pod") {
		if len(rule) <= 1 {
			return true
		} else {
			for i := 1; i < len(rule); i++ {
				if !strings.HasPrefix(rule[i].Name, "autogen-") {
					return true
				}
			}
		}
	}
	return false
}
