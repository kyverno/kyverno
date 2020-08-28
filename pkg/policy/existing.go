package policy

import (
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine"
	"github.com/nirmata/kyverno/pkg/engine/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (pc *PolicyController) processExistingResources(policy *kyverno.ClusterPolicy) []response.EngineResponse {
	logger := pc.log.WithValues("policy", policy.Name)
	// Parse through all the resources
	// drops the cache after configured rebuild time
	pc.rm.Drop()
	var engineResponses []response.EngineResponse
	// get resource that are satisfy the resource description defined in the rules
	resourceMap := pc.listResources(policy)
	for _, resource := range resourceMap {
		// pre-processing, check if the policy and resource version has been processed before
		if !pc.rm.ProcessResource(policy.Name, policy.ResourceVersion, resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion()) {
			logger.V(4).Info("policy and resource already processed", "policyResourceVersion", policy.ResourceVersion, "resourceResourceVersion", resource.GetResourceVersion(), "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
			continue
		}

		// skip reporting violation on pod which has annotation pod-policies.kyverno.io/autogen-applied
		ann := policy.GetAnnotations()
		if annValue, ok := ann[engine.PodControllersAnnotation]; ok {
			if annValue != "none" {
				if skipPodApplication(resource, logger) {
					continue
				}
			}
		}

		// apply the policy on each
		engineResponse := applyPolicy(*policy, resource, logger, pc.configHandler.GetExcludeGroupRole())
		// get engine response for mutation & validation independently
		engineResponses = append(engineResponses, engineResponse...)
		// post-processing, register the resource as processed
		pc.rm.RegisterResource(policy.GetName(), policy.GetResourceVersion(), resource.GetKind(), resource.GetNamespace(), resource.GetName(), resource.GetResourceVersion())
	}
	return engineResponses
}

func (pc *PolicyController) listResources(policy *kyverno.ClusterPolicy) map[string]unstructured.Unstructured {
	pc.log.V(4).Info("list resources to be processed")

	// key uid
	resourceMap := map[string]unstructured.Unstructured{}

	for _, rule := range policy.Spec.Rules {
		for _, k := range rule.MatchResources.Kinds {

			resourceSchema, _, err := pc.client.DiscoveryClient.FindResource("", k)
			if err != nil {
				pc.log.Error(err, "failed to find resource", "kind", k)
				continue
			}

			if !resourceSchema.Namespaced {
				rMap := GetResourcesPerNamespace(k, pc.client, "", rule, pc.configHandler, pc.log)
				MergeResources(resourceMap, rMap)
			} else {
				namespaces := GetNamespacesForRule(&rule, pc.nsLister, pc.log)
				for _, ns := range namespaces {
					rMap := GetResourcesPerNamespace(k, pc.client, ns, rule, pc.configHandler, pc.log)
					MergeResources(resourceMap, rMap)
				}
			}
		}
	}

	if policy.HasAutoGenAnnotation() {
		return ExcludePod(resourceMap, pc.log)
	}

	return resourceMap
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

// merge b into a map
func mergeResources(a, b map[string]unstructured.Unstructured) {
	for k, v := range b {
		a[k] = v
	}
}

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

func skipPodApplication(resource unstructured.Unstructured, log logr.Logger) bool {
	if resource.GetKind() != "Pod" {
		return false
	}

	annotation := resource.GetAnnotations()
	if _, ok := annotation[engine.PodTemplateAnnotation]; ok {
		log.V(4).Info("Policies already processed on pod controllers, skip processing policy on Pod", "kind", resource.GetKind(), "namespace", resource.GetNamespace(), "name", resource.GetName())
		return true
	}

	return false
}
