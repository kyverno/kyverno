package policy

import (
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/nirmata/kyverno/pkg/client/clientset/versioned"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type statusCache struct {
	mu   sync.RWMutex
	data map[string]v1.PolicyStatus
}

func (c *statusCache) Get(key string) v1.PolicyStatus {
	c.mu.RLock()
	status := c.data[key]
	c.mu.RUnlock()
	return status

}

func (c *statusCache) GetAll() map[string]v1.PolicyStatus {
	c.mu.RLock()
	mapCopy := make(map[string]v1.PolicyStatus, len(c.data))
	for k, v := range c.data {
		mapCopy[k] = v
	}
	c.mu.RUnlock()
	return mapCopy

}
func (c *statusCache) Set(key string, status v1.PolicyStatus) {
	c.mu.Lock()
	c.data[key] = status
	c.mu.Unlock()
}
func (c *statusCache) Clear() {
	c.mu.Lock()
	c.data = make(map[string]v1.PolicyStatus)
	c.mu.Unlock()
}

func newStatusCache() *statusCache {
	return &statusCache{
		mu:   sync.RWMutex{},
		data: make(map[string]v1.PolicyStatus),
	}
}

func NewStatusSync(client *versioned.Clientset, stopCh <-chan struct{}) *StatusSync {
	return &StatusSync{
		statusReceiver: make(chan map[string]v1.PolicyStatus),
		cache:          newStatusCache(),
		stop:           stopCh,
		client:         client,
	}
}

type StatusSync struct {
	statusReceiver chan map[string]v1.PolicyStatus
	cache          *statusCache
	stop           <-chan struct{}
	client         *versioned.Clientset
}

func (s *StatusSync) Cache() *statusCache {
	return s.cache
}

func (s *StatusSync) Receiver() chan<- map[string]v1.PolicyStatus {
	return s.statusReceiver
}

func (s *StatusSync) Start() {
	// receive status and store it in cache
	go func() {
		for {
			select {
			case nameToStatus := <-s.statusReceiver:
				for policyName, status := range nameToStatus {
					s.cache.Set(policyName, status)
				}
			case <-s.stop:
				return
			}
		}
	}()

	// update policy status every 10 seconds - waits for previous updateStatus to complete
	wait.Until(s.updateStatus, 10*time.Second, s.stop)
	<-s.stop
}

func (s *StatusSync) updateStatus() {
	for policyName, status := range s.cache.GetAll() {
		var policy = &v1.ClusterPolicy{}
		policy.Name = policyName
		policy.Status = status
		_, _ = s.client.KyvernoV1().ClusterPolicies().UpdateStatus(policy)
	}
	s.cache.Clear()
}
