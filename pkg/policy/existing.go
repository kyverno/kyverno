package policy

import (
	"errors"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// excludeAutoGenResources filter out the pods / jobs with ownerReference
func excludeAutoGenResources(policy kyvernov1.PolicyInterface, resourceMap map[string]unstructured.Unstructured, log logr.Logger) {
	for uid, r := range resourceMap {
		if engine.ManagedPodResource(policy, r) {
			log.V(4).Info("exclude resource", "namespace", r.GetNamespace(), "kind", r.GetKind(), "name", r.GetName())
			delete(resourceMap, uid)
		}
	}
}

// Condition defines condition type
type Condition int

const (
	// NotEvaluate to not evaluate condition
	NotEvaluate Condition = 0
	// Process to evaluate condition
	Process Condition = 1
	// Skip to ignore/skip the condition
	Skip Condition = 2
)

// NewResourceManager returns a new ResourceManager
func NewResourceManager(rebuildTime int64) *ResourceManager {
	rm := ResourceManager{
		scope:       make(map[string]bool),
		data:        make(map[string]interface{}),
		time:        time.Now(),
		rebuildTime: rebuildTime,
	}
	// set time it was built
	return &rm
}

// ResourceManager stores the details on already processed resources for caching
type ResourceManager struct {
	// we drop and re-build the cache
	// based on the memory consumer of by the map
	scope       map[string]bool
	data        map[string]interface{}
	mux         sync.RWMutex
	time        time.Time
	rebuildTime int64 // after how many seconds should we rebuild the cache
}

type resourceManager interface {
	ProcessResource(policy, pv, kind, ns, name, rv string) bool
	// TODO	removeResource(kind, ns, name string) error
	RegisterResource(policy, pv, kind, ns, name, rv string)
	RegisterScope(kind string, namespaced bool)
	GetScope(kind string) (bool, error)
	Drop()
}

// Drop drop the cache after every rebuild interval mins
func (rm *ResourceManager) Drop() {
	timeSince := time.Since(rm.time)
	if timeSince > time.Duration(rm.rebuildTime)*time.Second {
		rm.mux.Lock()
		defer rm.mux.Unlock()
		rm.data = map[string]interface{}{}
		rm.time = time.Now()
	}
}

var empty struct{}

// RegisterResource stores if the policy is processed on this resource version
func (rm *ResourceManager) RegisterResource(policy, pv, kind, ns, name, rv string) {
	rm.mux.Lock()
	defer rm.mux.Unlock()
	// add the resource
	key := buildKey(policy, pv, kind, ns, name, rv)
	rm.data[key] = empty
}

// ProcessResource returns true if the policy was not applied on the resource
func (rm *ResourceManager) ProcessResource(policy, pv, kind, ns, name, rv string) bool {
	rm.mux.RLock()
	defer rm.mux.RUnlock()

	key := buildKey(policy, pv, kind, ns, name, rv)
	_, ok := rm.data[key]
	return !ok
}

// RegisterScope stores the scope of the given kind
func (rm *ResourceManager) RegisterScope(kind string, namespaced bool) {
	rm.mux.Lock()
	defer rm.mux.Unlock()

	rm.scope[kind] = namespaced
}

// GetScope gets the scope of the given kind
// return error if kind is not registered
func (rm *ResourceManager) GetScope(kind string) (bool, error) {
	rm.mux.RLock()
	defer rm.mux.RUnlock()

	namespaced, ok := rm.scope[kind]
	if !ok {
		return false, errors.New("NotFound")
	}

	return namespaced, nil
}

func buildKey(policy, pv, kind, ns, name, rv string) string {
	return policy + "/" + pv + "/" + kind + "/" + ns + "/" + name + "/" + rv
}
