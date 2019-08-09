package engine

import (
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1alpha1"
	client "github.com/nirmata/kyverno/pkg/dclient"
	"github.com/nirmata/kyverno/pkg/info"
	"github.com/nirmata/kyverno/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProcessExisting checks for mutation and validation violations of existing resources
func ProcessExisting(client *client.Client, policy *kyverno.Policy, filterK8Resources []utils.K8Resource) []info.PolicyInfo {
	glog.Infof("Applying policy %s on existing resources", policy.Name)
	// key uid
	resourceMap := ListResourcesThatApplyToPolicy(client, policy, filterK8Resources)
	policyInfos := []info.PolicyInfo{}
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

func applyPolicy(client *client.Client, policy *kyverno.Policy, res resourceInfo) (info.PolicyInfo, error) {
	var policyInfo info.PolicyInfo
	glog.Infof("Applying policy %s with %d rules\n", policy.ObjectMeta.Name, len(policy.Spec.Rules))
	rawResource, err := res.Resource.MarshalJSON()
	if err != nil {
		return policyInfo, err
	}
	// Mutate
	mruleInfos, err := mutation(policy, rawResource, res.Gvk)
	policyInfo.AddRuleInfos(mruleInfos)
	if err != nil {
		return policyInfo, err
	}
	// Validation
	//TODO check by value or pointer
	vruleInfos, err := Validate(*policy, rawResource, *res.Gvk)
	if err != nil {
		return policyInfo, err
	}
	policyInfo.AddRuleInfos(vruleInfos)
	policyInfo = info.NewPolicyInfo(policy.Name, res.Gvk.Kind, res.Resource.GetName(), res.Resource.GetNamespace(), policy.Spec.ValidationFailureAction)

	if res.Gvk.Kind == "Namespace" {

		// Generation
		gruleInfos := Generate(client, policy, res.Resource)
		policyInfo.AddRuleInfos(gruleInfos)
	}

	return policyInfo, nil
}

func mutation(p *kyverno.Policy, rawResource []byte, gvk *metav1.GroupVersionKind) ([]*info.RuleInfo, error) {
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
