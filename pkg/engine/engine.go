package engine

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	"github.com/minio/minio/pkg/wildcard"
	types "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// As the logic to process the policies in stateless, we do not need to define struct and implement behaviors for it
// Instead we expose them as standalone functions passing the required atrributes
// The each function returns the changes that need to be applied on the resource
// the caller is responsible to apply the changes to the resource

// ProcessExisting checks for mutation and validation violations of existing resources
func ProcessExisting(client *client.Client, policy *types.Policy) []*info.PolicyInfo {
	glog.Infof("Applying policy %s on existing resources", policy.Name)
	resources := []*resourceInfo{}

	for _, rule := range policy.Spec.Rules {
		for _, k := range rule.Kinds {
			if k == "Namespace" {
				// REWORK: will be handeled by namespace controller
				continue
			}
			// kind -> resource
			gvr := client.DiscoveryClient.GetGVRFromKind(k)
			// label selectors
			// namespace ? should it be default or allow policy to specify it
			namespace := "default"
			if rule.ResourceDescription.Namespace != nil {
				namespace = *rule.ResourceDescription.Namespace
			}
			list, err := client.ListResource(k, namespace, rule.ResourceDescription.Selector)
			if err != nil {
				glog.Errorf("unable to list resource for %s with label selector %s", gvr.Resource, rule.Selector.String())
				glog.Errorf("unable to apply policy %s rule %s. err: %s", policy.Name, rule.Name, err)
				continue
			}
			for _, res := range list.Items {
				name := rule.ResourceDescription.Name
				gvk := res.GroupVersionKind()
				if name != nil {
					// wild card matching
					if !wildcard.Match(*name, res.GetName()) {
						continue
					}
				}
				ri := &resourceInfo{resource: &res, gvk: &metav1.GroupVersionKind{Group: gvk.Group,
					Version: gvk.Version,
					Kind:    gvk.Kind}}
				resources = append(resources, ri)

			}
		}
	}
	policyInfos := []*info.PolicyInfo{}
	// for the filtered resource apply policy
	for _, r := range resources {

		policyInfo, err := applyPolicy(client, policy, r)
		if err != nil {
			glog.Errorf("unable to apply policy %s on resource %s/%s", policy.Name, r.resource.GetName(), r.resource.GetNamespace())
			glog.Error(err)
			continue
		}
		policyInfos = append(policyInfos, policyInfo)
	}

	return policyInfos
}

func applyPolicy(client *client.Client, policy *types.Policy, res *resourceInfo) (*info.PolicyInfo, error) {
	policyInfo := info.NewPolicyInfo(policy.Name, res.gvk.Kind, res.resource.GetName(), res.resource.GetNamespace(), policy.Spec.ValidationFailureAction)
	glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))
	rawResource, err := res.resource.MarshalJSON()
	if err != nil {
		return nil, err
	}
	// Mutate
	mruleInfos, err := mutation(policy, rawResource, res.gvk)
	policyInfo.AddRuleInfos(mruleInfos)
	if err != nil {
		return nil, err
	}
	// Validation
	vruleInfos, err := Validate(*policy, rawResource, *res.gvk)
	policyInfo.AddRuleInfos(vruleInfos)
	if err != nil {
		return nil, err
	}
	// Generate rule managed by generation controller

	return policyInfo, nil
}

func mutation(p *types.Policy, rawResource []byte, gvk *metav1.GroupVersionKind) ([]*info.RuleInfo, error) {
	patches, ruleInfos := Mutate(*p, rawResource, *gvk)
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
