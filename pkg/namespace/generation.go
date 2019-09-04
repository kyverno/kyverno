package namespace

import (
	"sync"
	"time"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	kyvernolister "github.com/nirmata/kyverno/pkg/client/listers/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"
	policyctr "github.com/nirmata/kyverno/pkg/policy"
	"github.com/nirmata/kyverno/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
)

type resourceManager interface {
	ProcessResource(policy, pv, kind, ns, name, rv string) bool
	//TODO	removeResource(kind, ns, name string) error
	RegisterResource(policy, pv, kind, ns, name, rv string)
	// reload
	Drop()
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
	return ok == false
}

//Drop drop the cache after every rebuild interval mins
//TODO: or drop based on the size
func (rm *ResourceManager) Drop() {
	timeSince := time.Since(rm.time)
	glog.V(4).Infof("time since last cache reset time %v is %v", rm.time, timeSince)
	glog.V(4).Infof("cache rebuild time %v", time.Duration(rm.rebuildTime)*time.Second)
	if timeSince > time.Duration(rm.rebuildTime)*time.Second {
		rm.mux.Lock()
		defer rm.mux.Unlock()
		rm.data = map[string]interface{}{}
		rm.time = time.Now()
		glog.V(4).Infof("dropping cache at time %v", rm.time)
	}
}
func buildKey(policy, pv, kind, ns, name, rv string) string {
	return policy + "/" + pv + "/" + kind + "/" + ns + "/" + name + "/" + rv
}

func (nsc *NamespaceController) processNamespace(namespace corev1.Namespace) []engine.EngineResponseNew {
	//	convert to unstructured
	unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&namespace)
	if err != nil {
		glog.Infof("unable to convert to unstructured, not processing any policies: %v", err)
		return nil
	}
	nsc.rm.Drop()

	ns := unstructured.Unstructured{Object: unstr}

	// get all the policies that have a generate rule and resource description satifies the namespace
	// apply policy on resource
	policies := listpolicies(ns, nsc.pLister)
	var engineResponses []engine.EngineResponseNew
	for _, policy := range policies {
		// pre-processing, check if the policy and resource version has been processed before
		if !nsc.rm.ProcessResource(policy.Name, policy.ResourceVersion, ns.GetKind(), ns.GetNamespace(), ns.GetName(), ns.GetResourceVersion()) {
			glog.V(4).Infof("policy %s with resource version %s already processed on resource %s/%s/%s with resource version %s", policy.Name, policy.ResourceVersion, ns.GetKind(), ns.GetNamespace(), ns.GetName(), ns.GetResourceVersion())
			continue
		}
		engineResponse := applyPolicy(nsc.client, ns, *policy, nsc.policyStatus)
		engineResponses = append(engineResponses, engineResponse)

		// post-processing, register the resource as processed
		nsc.rm.RegisterResource(policy.GetName(), policy.GetResourceVersion(), ns.GetKind(), ns.GetNamespace(), ns.GetName(), ns.GetResourceVersion())
	}
	return engineResponses
}

func listpolicies(ns unstructured.Unstructured, pLister kyvernolister.ClusterPolicyLister) []*kyverno.ClusterPolicy {
	var filteredpolicies []*kyverno.ClusterPolicy
	glog.V(4).Infof("listing policies for namespace %s", ns.GetName())
	policies, err := pLister.List(labels.NewSelector())
	if err != nil {
		glog.Errorf("failed to get list policies: %v", err)
		return nil
	}
	for _, policy := range policies {
		for _, rule := range policy.Spec.Rules {
			if rule.Generation == (kyverno.Generation{}) {
				continue
			}
			ok := engine.MatchesResourceDescription(ns, rule)
			if !ok {
				glog.V(4).Infof("namespace %s does not satisfy the resource description for the policy %s rule %s", ns.GetName(), policy.Name, rule.Name)
				continue
			}
			glog.V(4).Infof("namespace %s satisfies resource description for policy %s rule %s", ns.GetName(), policy.Name, rule.Name)
			filteredpolicies = append(filteredpolicies, policy)
		}
	}
	return filteredpolicies
}

func applyPolicy(client *client.Client, resource unstructured.Unstructured, p kyverno.ClusterPolicy, policyStatus policyctr.PolicyStatusInterface) engine.EngineResponseNew {
	var policyStats []policyctr.PolicyStat
	// gather stats from the engine response
	gatherStat := func(policyName string, policyResponse engine.PolicyResponse) {
		ps := policyctr.PolicyStat{}
		ps.PolicyName = policyName
		ps.Stats.MutationExecutionTime = policyResponse.ProcessingTime
		ps.Stats.RulesAppliedCount = policyResponse.RulesAppliedCount
		// capture rule level stats
		for _, rule := range policyResponse.Rules {
			rs := policyctr.RuleStatinfo{}
			rs.RuleName = rule.Name
			rs.ExecutionTime = rule.RuleStats.ProcessingTime
			if rule.Success {
				rs.RuleAppliedCount++
			} else {
				rs.RulesFailedCount++
			}
			ps.Stats.Rules = append(ps.Stats.Rules, rs)
		}
		policyStats = append(policyStats, ps)
	}
	// send stats for aggregation
	sendStat := func(blocked bool) {
		for _, stat := range policyStats {
			stat.Stats.ResourceBlocked = utils.Btoi(blocked)
			//SEND
			policyStatus.SendStat(stat)
		}
	}

	startTime := time.Now()
	glog.V(4).Infof("Started apply policy %s on resource %s/%s/%s (%v)", p.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), startTime)
	defer func() {
		glog.V(4).Infof("Finished applying %s on resource %s/%s/%s (%v)", p.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), time.Since(startTime))
	}()
	engineResponse := engine.Generate(client, p, resource)
	// gather stats
	gatherStat(p.Name, engineResponse.PolicyResponse)
	//send stats
	sendStat(false)

	return engineResponse
}
