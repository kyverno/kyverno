package policyStatus

import (
	"sync"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type statusUpdater interface {
	UpdateStatus(s *Sync)
}

type policyStore interface {
	Get(policyName string) (*v1.ClusterPolicy, error)
}

type Sync struct {
	Cache       *cache
	Listener    chan statusUpdater
	client      *versioned.Clientset
	PolicyStore policyStore
}

type cache struct {
	Mutex sync.RWMutex
	Data  map[string]v1.PolicyStatus
}

func NewSync(c *versioned.Clientset, p policyStore) *Sync {
	return &Sync{
		Cache: &cache{
			Mutex: sync.RWMutex{},
			Data:  make(map[string]v1.PolicyStatus),
		},
		client:      c,
		PolicyStore: p,
		Listener:    make(chan statusUpdater),
	}
}

func (s *Sync) Run(workers int, stopCh <-chan struct{}) {
	for i := 0; i < workers; i++ {
		go s.updateStatusCache(stopCh)
	}

	wait.Until(s.updatePolicyStatus, 2*time.Second, stopCh)
	<-stopCh
	s.updatePolicyStatus()
}

func (s *Sync) updateStatusCache(stopCh <-chan struct{}) {
	for {
		select {
		case statusUpdater := <-s.Listener:
			statusUpdater.UpdateStatus(s)
		case <-stopCh:
			return
		}
	}
}

func (s *Sync) updatePolicyStatus() {
	s.Cache.Mutex.Lock()
	var nameToStatus = make(map[string]v1.PolicyStatus, len(s.Cache.Data))
	for k, v := range s.Cache.Data {
		nameToStatus[k] = v
	}
	s.Cache.Mutex.Unlock()

	for policyName, status := range nameToStatus {
		policy, err := s.PolicyStore.Get(policyName)
		if err != nil {
			continue
		}
		policy.Status = status
		_, err = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(policy)
		if err != nil {
			s.Cache.Mutex.Lock()
			delete(s.Cache.Data, policyName)
			s.Cache.Mutex.Unlock()
			glog.V(4).Info(err)
		}
	}
}
