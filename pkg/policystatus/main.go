package policystatus

import (
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type statusUpdater interface {
	PolicyName() string
	UpdateStatus(status v1.PolicyStatus) v1.PolicyStatus
}

type policyStore interface {
	Get(policyName string) (*v1.ClusterPolicy, error)
}

type Listener chan statusUpdater

func (l Listener) Send(s statusUpdater) {
	l <- s
}

type Sync struct {
	cache       *cache
	Listener    Listener
	client      *versioned.Clientset
	policyStore policyStore
}

type cache struct {
	dataMu     sync.RWMutex
	data       map[string]v1.PolicyStatus
	keyToMutex *keyToMutex
}

func NewSync(c *versioned.Clientset, p policyStore) *Sync {
	return &Sync{
		cache: &cache{
			dataMu:     sync.RWMutex{},
			data:       make(map[string]v1.PolicyStatus),
			keyToMutex: newKeyToMutex(),
		},
		client:      c,
		policyStore: p,
		Listener:    make(chan statusUpdater, 20),
	}
}

func (s *Sync) Run(workers int, stopCh <-chan struct{}) {
	for i := 0; i < workers; i++ {
		go s.updateStatusCache(stopCh)
	}

	wait.Until(s.updatePolicyStatus, 2*time.Second, stopCh)
	<-stopCh
}

func (s *Sync) updateStatusCache(stopCh <-chan struct{}) {
	for {
		select {
		case statusUpdater := <-s.Listener:
			s.cache.keyToMutex.Get(statusUpdater.PolicyName()).Lock()

			s.cache.dataMu.RLock()
			status, exist := s.cache.data[statusUpdater.PolicyName()]
			s.cache.dataMu.RUnlock()
			if !exist {
				policy, _ := s.policyStore.Get(statusUpdater.PolicyName())
				if policy != nil {
					status = policy.Status
				}
			}

			updatedStatus := statusUpdater.UpdateStatus(status)

			s.cache.dataMu.Lock()
			s.cache.data[statusUpdater.PolicyName()] = updatedStatus
			s.cache.dataMu.Unlock()

			s.cache.keyToMutex.Get(statusUpdater.PolicyName()).Unlock()
		case <-stopCh:
			return
		}
	}
}

func (s *Sync) updatePolicyStatus() {
	s.cache.dataMu.Lock()
	var nameToStatus = make(map[string]v1.PolicyStatus, len(s.cache.data))
	for k, v := range s.cache.data {
		nameToStatus[k] = v
	}
	s.cache.dataMu.Unlock()

	for policyName, status := range nameToStatus {
		policy, err := s.policyStore.Get(policyName)
		if err != nil {
			continue
		}
		policy.Status = status
		_, err = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(policy)
		if err != nil {
			s.cache.dataMu.Lock()
			delete(s.cache.data, policyName)
			s.cache.dataMu.Unlock()
			glog.V(4).Info(err)
		}
	}
}
