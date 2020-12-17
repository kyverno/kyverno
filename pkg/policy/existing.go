package policy

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/kyverno/kyverno/pkg/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine"
	"github.com/kyverno/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (pc *PolicyController) processExistingResources(policy *kyverno.ClusterPolicy) {
	logger := pc.log.WithValues("policy", policy.Name)
	logger.V(4).Info("applying policy to existing resources")

	// Parse through all the resources drops the cache after configured rebuild time
	pc.rm.Drop()

	for _, rule := range policy.Spec.Rules {
		for _, k := range rule.MatchResources.Kinds {
			logger = logger.WithValues("rule", rule.Name, "kind", k)
			resourceSchema, _, err := pc.client.DiscoveryClient.FindResource("", k)
			if err != nil {
				logger.Error(err, "failed to find resource", "kind", k)
				continue
			}

			if !resourceSchema.Namespaced {
				pc.applyAndReportPerNamespace(policy, k, "", rule, logger)
				continue
			}

			namespaces := GetNamespacesForRule(&rule, pc.nsLister, logger)
			for _, ns := range namespaces {
				pc.applyAndReportPerNamespace(policy, k, ns, rule, logger)
			}
		}
	}
}

func (pc *PolicyController) applyAndReportPerNamespace(policy *kyverno.ClusterPolicy, kind string, ns string, rule kyverno.Rule, logger logr.Logger) {
	rMap := pc.getResourcesPerNamespace(kind, ns, rule, logger)
	excludeAutoGenResources(*policy, rMap, logger)

	if len(rMap) == 0 {
		return
	}

	var engineResponses []response.EngineResponse
	for _, resource := range rMap {
		responses := pc.applyPolicy(policy, resource, logger)
		engineResponses = append(engineResponses, responses...)
	}

	pc.report(policy.Name, engineResponses, logger)
}

func (pc *PolicyController) applyPolicy(policy *kyverno.ClusterPolicy, resource unstructured.Unstructured, logger logr.Logger) (engineResponses []response.EngineResponse) {
	// pre-processing, check if the policy and resource version has been processed before
	if !pc.rm.ProcessResource(policy.Name, policy.ResourceVersion, resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion()) {
		logger.V(4).Info("policy and resource already processed", "policyResourceVersion", policy.ResourceVersion, "resourceResourceVersion", resource.GetResourceVersion(), "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
	}

	engineResponse := applyPolicy(*policy, resource, logger, pc.configHandler.GetExcludeGroupRole(), pc.resCache)
	engineResponses = append(engineResponses, engineResponse...)

	// post-processing, register the resource as processed
	pc.rm.RegisterResource(policy.GetName(), policy.GetResourceVersion(), resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion())

	return
}

// excludeAutoGenResources filter out the pods / jobs with ownerReference
func excludeAutoGenResources(policy kyverno.ClusterPolicy, resourceMap map[string]unstructured.Unstructured, log logr.Logger) {
	for uid, r := range resourceMap {
		if engine.SkipPolicyApplication(policy, r) {
			log.V(4).Info("exclude resource", "namespace", r.GetNamespace(), "kind", r.GetKind(), "name", r.GetName())
			delete(resourceMap, uid)
		}
	}
}

//Condition defines condition type
type Condition int

const (
	//NotEvaluate to not evaluate condition
	NotEvaluate Condition = 0
	// Process to evaluate condition
	Process Condition = 1
	// Skip to ignore/skip the condition
	Skip Condition = 2
)

//NewResourceManager returns a new ResourceManager
func NewResourceManager(rebuildTime int64) *ResourceManager {
	rm := ResourceManager{
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
	data        map[string]interface{}
	mux         sync.RWMutex
	time        time.Time
	rebuildTime int64 // after how many seconds should we rebuild the cache
}

type resourceManager interface {
	ProcessResource(policy, pv, kind, ns, name, rv string) bool
	//TODO	removeResource(kind, ns, name string) error
	RegisterResource(policy, pv, kind, ns, name, rv string)
	// reload
	Drop()
}

//Drop drop the cache after every rebuild interval mins
//TODO: or drop based on the size
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

//RegisterResource stores if the policy is processed on this resource version
func (rm *ResourceManager) RegisterResource(policy, pv, kind, ns, name, rv string) {
	rm.mux.Lock()
	defer rm.mux.Unlock()
	// add the resource
	key := buildKey(policy, pv, kind, ns, name, rv)
	rm.data[key] = empty
}

//ProcessResource returns true if the policy was not applied on the resource
func (rm *ResourceManager) ProcessResource(policy, pv, kind, ns, name, rv string) bool {
	rm.mux.RLock()
	defer rm.mux.RUnlock()

	key := buildKey(policy, pv, kind, ns, name, rv)
	_, ok := rm.data[key]
	return !ok
}

func buildKey(policy, pv, kind, ns, name, rv string) string {
	return policy + "/" + pv + "/" + kind + "/" + ns + "/" + name + "/" + rv
}
