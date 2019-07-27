package engine

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	"github.com/minio/minio/pkg/wildcard"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1helper "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func ListResourcesThatApplyToPolicy(client *client.Client, policy *types.Policy, filterK8Resources []utils.K8Resource) map[string]resourceInfo {
	// key uid
	resourceMap := map[string]resourceInfo{}
	for _, rule := range policy.Spec.Rules {
		// Match
		for _, k := range rule.MatchResources.Kinds {
			// kind -> resource
			gvr := client.DiscoveryClient.GetGVRFromKind(k)
			// Namespace
			namespace := "default"
			if rule.MatchResources.Namespace != nil {
				namespace = *rule.MatchResources.Namespace
			}
			if k == "Namespace" {
				namespace = ""
			}
			// Check if exclude namespace is not clashing
			if rule.ExcludeResources.Namespace != nil && *rule.ExcludeResources.Namespace == namespace {
				// as the namespace is excluded
				continue
			}

			// List resources
			list, err := client.ListResource(k, namespace, rule.MatchResources.Selector)
			if err != nil {
				glog.Errorf("unable to list resource for %s with label selector %s", gvr.Resource, rule.MatchResources.Selector.String())
				glog.Errorf("unable to apply policy %s rule %s. err: %s", policy.Name, rule.Name, err)
				continue
			}
			var selector labels.Selector
			// exclude label selector
			if rule.ExcludeResources.Selector != nil {
				selector, err = v1helper.LabelSelectorAsSelector(rule.ExcludeResources.Selector)
				if err != nil {
					glog.Error(err)
				}
			}
			for _, res := range list.Items {
				// exclude label selectors
				if selector != nil {
					set := labels.Set(res.GetLabels())
					if selector.Matches(set) {
						// if matches
						continue
					}
				}
				var name *string
				// match
				// name
				// wild card matching
				name = rule.MatchResources.Name
				if name != nil {
					// if does not match then we skip
					if !wildcard.Match(*name, res.GetName()) {
						continue
					}
				}
				// exclude
				// name
				// wild card matching
				name = rule.ExcludeResources.Name
				if name != nil {
					// if matches then we skip
					if wildcard.Match(*name, res.GetName()) {
						continue
					}
				}
				gvk := res.GroupVersionKind()

				ri := resourceInfo{Resource: res, Gvk: &metav1.GroupVersionKind{Group: gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind}}
				// Skip the filtered resources
				if utils.SkipFilteredResources(gvk.Kind, res.GetNamespace(), res.GetName(), filterK8Resources) {
					continue
				}
				resourceMap[string(res.GetUID())] = ri

			}
		}
	}
	return resourceMap
}

// ProcessExisting checks for mutation and validation violations of existing resources
func ProcessExisting(client *client.Client, policy *types.Policy, filterK8Resources []utils.K8Resource) []*info.PolicyInfo {
	glog.Infof("Applying policy %s on existing resources", policy.Name)
	// key uid
	resourceMap := ListResourcesThatApplyToPolicy(client, policy, filterK8Resources)
	policyInfos := []*info.PolicyInfo{}
	// for the filtered resource apply policy
	for _, v := range resourceMap {

		policyInfo, err := applyPolicy(client, policy, v)
		if err != nil {
			glog.Errorf("unable to apply policy %s on resource %s/%s", policy.Name, v.Resource.GetName(), v.Resource.GetNamespace())
			glog.Error(err)
			continue
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	return policyInfos
}

func applyPolicy(client *client.Client, policy *types.Policy, res resourceInfo) (*info.PolicyInfo, error) {
	policyInfo := info.NewPolicyInfo(policy.Name, res.Gvk.Kind, res.Resource.GetName(), res.Resource.GetNamespace(), policy.Spec.ValidationFailureAction)
	glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))
	rawResource, err := res.Resource.MarshalJSON()
	if err != nil {
		return nil, err
	}
	// Mutate
	mruleInfos, err := mutation(policy, rawResource, res.Gvk)
	policyInfo.AddRuleInfos(mruleInfos)
	if err != nil {
		return nil, err
	}
	// Validation
	vruleInfos, err := Validate(*policy, rawResource, *res.Gvk)
	policyInfo.AddRuleInfos(vruleInfos)
	if err != nil {
		return nil, err
	}
	if res.Gvk.Kind == "Namespace" {

		// Generation
		gruleInfos := Generate(client, policy, res.Resource)
		policyInfo.AddRuleInfos(gruleInfos)
	}

	return policyInfo, nil
}

func mutation(p *types.Policy, rawResource []byte, gvk *metav1.GroupVersionKind) ([]*info.RuleInfo, error) {
	patches, ruleInfos := Mutate(*p, rawResource, *gvk)
	if len(ruleInfos) == 0 {
		// no rules were processed
		return nil, nil
	}
	// if there are any errors return
	for _, r := range ruleInfos {
		if !r.IsSuccessful() {
			return ruleInfos, nil
		}
	}
	// if there are no patches // for overlay
	if len(patches) == 0 {
		return ruleInfos, nil
	}
	// option 2: (original Resource + patch) compare with (original resource)
	mergePatches := JoinPatches(patches)
	// merge the patches
	patch, err := jsonpatch.DecodePatch(mergePatches)
	if err != nil {
		return nil, err
	}
	// apply the patches returned by mutate to the original resource
	patchedResource, err := patch.Apply(rawResource)
	if err != nil {
		return nil, err
	}
	// compare (original Resource + patch) vs (original resource)
	// to verify if they are equal
	ruleInfo := info.NewRuleInfo("over-all mutation", info.Mutation)
	if !jsonpatch.Equal(patchedResource, rawResource) {
		//resource does not match so there was a mutation rule violated
		// TODO : check the rule name "mutation rules"
		ruleInfo.Fail()
		ruleInfo.Add("resource does not satisfy mutation rules")
	} else {
		ruleInfo.Add("resource satisfys the mutation rule")
	}
	ruleInfos = append(ruleInfos, ruleInfo)
	return ruleInfos, nil
}
