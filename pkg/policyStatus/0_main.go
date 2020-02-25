package policyStatus

import (
	"sync"
	"time"

	"github.com/golang/glog"

	"github.com/nirmata/kyverno/pkg/policystore"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type Sync struct {
	cache       *cache
	listener    chan statusUpdater
	stop        <-chan struct{}
	client      *versioned.Clientset
	policyStore *policystore.PolicyStore
}

type cache struct {
	mutex sync.RWMutex
	data  map[string]v1.PolicyStatus
}

func NewSync(c *versioned.Clientset, sc <-chan struct{}, pms *policystore.PolicyStore) *Sync {
	return &Sync{
		cache: &cache{
			mutex: sync.RWMutex{},
			data:  make(map[string]v1.PolicyStatus),
		},
		stop:        sc,
		client:      c,
		policyStore: pms,
	}
}

func (s *Sync) Run() {
	wait.Until(s.updatePolicyStatus, 5*time.Second, s.stop)
	<-s.stop
	s.updatePolicyStatus()
}

func (s *Sync) updateStatusCache() {
	for {
		select {
		case statusUpdater := <-s.listener:
			statusUpdater.updateStatus()
		case <-s.stop:
			return
		}
	}
}

func (s *Sync) updatePolicyStatus() {
	s.cache.mutex.Lock()
	var nameToStatus = make(map[string]v1.PolicyStatus, len(s.cache.data))
	for k, v := range s.cache.data {
		nameToStatus[k] = v
	}
	s.cache.data = make(map[string]v1.PolicyStatus)
	s.cache.mutex.Unlock()

	for policyName, status := range nameToStatus {
		var policy = &v1.ClusterPolicy{}
		policy, err := s.policyStore.Get(policyName)
		if err != nil {
			continue
		}
		policy.Status = status
		_, err = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(policy)
		if err != nil {
			glog.V(4).Info(err)
		}
	}
}
