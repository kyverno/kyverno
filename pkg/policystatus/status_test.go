package policystatus

import (
	"encoding/json"
	"testing"
	"time"

	v1 "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
)

type dummyStore struct {
}

func (d dummyStore) Get(policyName string) (*v1.ClusterPolicy, error) {
	return &v1.ClusterPolicy{}, nil
}

type dummyStatusUpdater struct {
}

func (d dummyStatusUpdater) UpdateStatus(status v1.PolicyStatus) v1.PolicyStatus {
	status.RulesAppliedCount++
	return status
}

func (d dummyStatusUpdater) PolicyName() string {
	return "policy1"
}

func TestKeyToMutex(t *testing.T) {
	expectedCache := `{"policy1":{"averageExecutionTime":"","rulesAppliedCount":100}}`

	stopCh := make(chan struct{})
	s := NewSync(nil, dummyStore{})
	for i := 0; i < 100; i++ {
		go s.updateStatusCache(stopCh)
	}

	for i := 0; i < 100; i++ {
		go s.Listener.Send(dummyStatusUpdater{})
	}

	<-time.After(time.Second * 3)
	stopCh <- struct{}{}

	cacheRaw, _ := json.Marshal(s.cache.data)
	if string(cacheRaw) != expectedCache {
		t.Errorf("\nTestcase Failed\nGot:\n%v\nExpected:\n%v\n", string(cacheRaw), expectedCache)
	}
}
