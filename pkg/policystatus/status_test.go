package policystatus

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	v1 "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	lv1 "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
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

type dummyLister struct {
}

func (dl dummyLister) List(selector labels.Selector) (ret []*v1.ClusterPolicy, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyLister) Get(name string) (*v1.ClusterPolicy, error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyLister) ListResources(selector labels.Selector) (ret []*v1.ClusterPolicy, err error) {
	return nil, fmt.Errorf("not implemented")
}

// type dymmyNsNamespace struct {}

type dummyNsLister struct {
}

func (dl dummyNsLister) Policies(name string) lv1.PolicyNamespaceLister {
	return dummyNsLister{}
}

func (dl dummyNsLister) List(selector labels.Selector) (ret []*v1.Policy, err error) {
	return nil, fmt.Errorf("not implemented")
}

func (dl dummyNsLister) Get(name string) (*v1.Policy, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestKeyToMutex(t *testing.T) {
	expectedCache := `{"policy1":{"rulesAppliedCount":100}}`

	stopCh := make(chan struct{})
	s := NewSync(nil, dummyLister{}, dummyNsLister{})
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
