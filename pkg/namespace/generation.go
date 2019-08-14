package namespace

import (
	"sync"
	"time"

	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/engine"

	"github.com/golang/glog"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"

	lister "github.com/nirmata/kyverno/pkg/clientNew/listers/kyverno/v1alpha1"
	"github.com/nirmata/kyverno/pkg/info"
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

func buildKey(policy, pv, kind, ns, name, rv string) string {
	return policy + "/" + pv + "/" + kind + "/" + ns + "/" + name + "/" + rv
}

// func (nsc *NamespaceController) listPolicies(ns *corev1.Namespace) ([]*v1alpha1.Policy, error) {
// 	var fpolicies []*v1alpha1.Policy
// 	policies, err := c.policyLister.List(labels.NewSelector())
// 	if err != nil {
// 		glog.Error("Unable to connect to policy controller. Unable to access policies not applying GENERATION rules")
// 		return nil, err
// 	}
// 	for _, p := range policies {
// 		// Check if the policy contains a generatoin rule
// 		for _, r := range p.Spec.Rules {
// 			if r.Generation != nil {
// 				// Check if the resource meets the description
// 				data, err := json.Marshal(ns)
// 				if err != nil {
// 					glog.Error(err)
// 					continue
// 				}
// 				// convert types of GVK
// 				nsGvk := schema.FromAPIVersionAndKind("v1", "Namespace")
// 				// Hardcode as we have a informer on specified gvk
// 				gvk := metav1.GroupVersionKind{Group: nsGvk.Group, Kind: nsGvk.Kind, Version: nsGvk.Version}
// 				if engine.ResourceMeetsDescription(data, r.MatchResources.ResourceDescription, r.ExcludeResources.ResourceDescription, gvk) {
// 					fpolicies = append(fpolicies, p)
// 					break
// 				}
// 			}
// 		}
// 	}

// func (nsc *NamespaceController) processNamespace(ns *corev1.Namespace) error {
// 	//Get all policies and then verify if the namespace matches any of the defined selectors
// 	policies, err := c.listPolicies(ns)
// 	if err != nil {
// 		return err
// 	}
// 	// process policy on namespace
// 	for _, p := range policies {
// 		c.processPolicy(ns, p)
// 	}

// 	return nil
// }

func (nsc *NamespaceController) processNamespace(namespace corev1.Namespace) []info.PolicyInfo {
	var policyInfos []info.PolicyInfo
	//	convert to unstructured
	unstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(namespace)
	if err != nil {
		glog.Infof("unable to convert to unstructured, not processing any policies: %v", err)
		return policyInfos
	}
	ns := unstructured.Unstructured{Object: unstr}

	// get all the policies that have a generate rule and apply on the namespace
	// apply policy on resource

	policies := listpolicies(ns, nsc.pLister)
	for _, policy := range policies {
		policyInfo := applyPolicy(nsc.client, ns, *policy)
		policyInfos = append(policyInfos, policyInfo)
	}
	return policyInfos
}

func listpolicies(ns unstructured.Unstructured, pLister lister.PolicyLister) []*kyverno.Policy {
	var filteredpolicies []*kyverno.Policy
	glog.V(4).Infof("listing policies that namespace %s", ns.GetName())
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
				glog.V(4).Infof("namespace %s does not satisfy the resource description for the rule ", ns.GetName())
				continue
			}
			filteredpolicies = append(filteredpolicies, policy)
		}
	}

	return filteredpolicies
}

func applyPolicy(client *client.Client, resource unstructured.Unstructured, policy kyverno.Policy) info.PolicyInfo {
	startTime := time.Now()
	glog.V(4).Infof("Started apply policy %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), startTime)
	defer func() {
		glog.V(4).Infof("Finished applying %s on resource %s/%s/%s (%v)", policy.Name, resource.GetKind(), resource.GetNamespace(), resource.GetName(), time.Since(startTime))
	}()
	policyInfo := info.NewPolicyInfo(policy.Name, resource.GetKind(), resource.GetName(), resource.GetNamespace(), policy.Spec.ValidationFailureAction)
	ruleInfos := engine.Generate(client, policy, resource)
	policyInfo.AddRuleInfos(ruleInfos)

	return policyInfo
}

// // func (nsc *NamespaceController) listPolicies(ns *corev1.Namespace) ([]*v1alpha1.Policy, error) {
// // 	var fpolicies []*v1alpha1.Policy
// // 	policies, err := c.policyLister.List(labels.NewSelector())
// // 	if err != nil {
// // 		glog.Error("Unable to connect to policy controller. Unable to access policies not applying GENERATION rules")
// // 		return nil, err
// // 	}
// // 	for _, p := range policies {
// // 		// Check if the policy contains a generatoin rule
// // 		for _, r := range p.Spec.Rules {
// // 			if r.Generation != nil {
// // 				// Check if the resource meets the description
// // 				data, err := json.Marshal(ns)
// // 				if err != nil {
// // 					glog.Error(err)
// // 					continue
// // 				}
// // 				// convert types of GVK
// // 				nsGvk := schema.FromAPIVersionAndKind("v1", "Namespace")
// // 				// Hardcode as we have a informer on specified gvk
// // 				gvk := metav1.GroupVersionKind{Group: nsGvk.Group, Kind: nsGvk.Kind, Version: nsGvk.Version}
// // 				if engine.ResourceMeetsDescription(data, r.MatchResources.ResourceDescription, r.ExcludeResources.ResourceDescription, gvk) {
// // 					fpolicies = append(fpolicies, p)
// // 					break
// // 				}
// // 			}
// // 		}
// // 	}

// // 	return fpolicies, nil
// // }

// func (nsc *NamespaceController) processPolicy(ns *corev1.Namespace, p *v1alpha1.Policy) {
// 	var eventInfo *event.Info
// 	var onViolation bool
// 	var msg string

// 	policyInfo := info.NewPolicyInfo(p.Name,
// 		"Namespace",
// 		ns.Name,
// 		"",
// 		p.Spec.ValidationFailureAction) // Namespace has no namespace..WOW

// 	// convert to unstructured
// 	unstrMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
// 	if err != nil {
// 		glog.Error(err)
// 		return
// 	}
// 	unstObj := unstructured.Unstructured{Object: unstrMap}
// 	ruleInfos := engine.Generate(c.client, p, unstObj)
// 	policyInfo.AddRuleInfos(ruleInfos)

// 	// generate annotations on namespace
// 	c.createAnnotations(policyInfo)
// 	//TODO generate namespace on created resources

// 	if !policyInfo.IsSuccessful() {
// 		glog.Infof("Failed to apply policy %s on resource %s %s", p.Name, ns.Kind, ns.Name)
// 		for _, r := range ruleInfos {
// 			glog.Warning(r.Msgs)

// 			if msg = strings.Join(r.Msgs, " "); strings.Contains(msg, "rule configuration not present in resource") {
// 				onViolation = true
// 				msg = fmt.Sprintf(`Resource creation violates generate rule '%s' of policy '%s'`, r.Name, policyInfo.Name)
// 			}
// 		}

// 		if onViolation {
// 			glog.Infof("Adding violation for generation rule of policy %s\n", policyInfo.Name)
// 			// Policy Violation
// 			v := violation.BuldNewViolation(policyInfo.Name, policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName, event.PolicyViolation.String(), policyInfo.FailedRules())
// 			c.violationBuilder.Add(v)
// 		} else {
// 			// Event
// 			eventInfo = event.NewEvent(policyKind, "", policyInfo.Name, event.RequestBlocked,
// 				event.FPolicyApplyBlockCreate, policyInfo.RName, policyInfo.GetRuleNames(false))

// 			glog.V(2).Infof("Request blocked event info has prepared for %s/%s\n", policyKind, policyInfo.Name)

// 			c.eventController.Add(eventInfo)
// 		}
// 		return
// 	}

// 	glog.Infof("Generation from policy %s has succesfully applied to %s/%s", p.Name, policyInfo.RKind, policyInfo.RName)

// 	eventInfo = event.NewEvent(policyInfo.RKind, policyInfo.RNamespace, policyInfo.RName,
// 		event.PolicyApplied, event.SRulesApply, policyInfo.GetRuleNames(true), policyInfo.Name)

// 	glog.V(2).Infof("Success event info has prepared for %s/%s\n", policyInfo.RKind, policyInfo.RName)

// 	c.eventController.Add(eventInfo)
// }

// func (nsc *NamespaceController) createAnnotations(pi *info.PolicyInfo) {
// 	//get resource
// 	obj, err := c.client.GetResource(pi.RKind, pi.RNamespace, pi.RName)
// 	if err != nil {
// 		glog.Error(err)
// 		return
// 	}
// 	// add annotation for policy application
// 	ann := obj.GetAnnotations()
// 	// Generation rules
// 	ann, gpatch, err := annotations.AddPolicyJSONPatch(ann, pi, info.Generation)
// 	if err != nil {
// 		glog.Error(err)
// 		return
// 	}
// 	if gpatch == nil {
// 		// nothing to patch
// 		return
// 	}
// 	//		add the anotation to the resource
// 	_, err = c.client.PatchResource(pi.RKind, pi.RNamespace, pi.RName, gpatch)
// 	if err != nil {
// 		glog.Error(err)
// 		return
// 	}
// }
