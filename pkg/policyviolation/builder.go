package policyviolation

import (
	"fmt"

	"github.com/golang/glog"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

func GeneratePVsFromEngineResponse(ers []response.EngineResponse) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resoruces that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			glog.V(4).Infof("resource %v, has not been assigned a name, not creating a policy violation for it", er.PolicyResponse.Resource)
			continue
		}
		if er.IsSuccesful() {
			continue
		}
		glog.V(4).Infof("Building policy violation for engine response %v", er)
		// build policy violation info
		pvInfos = append(pvInfos, buildPVInfo(er))
	}

	return pvInfos
}

// Builder builds Policy Violation struct
// this is base type of namespaced and cluster policy violation
type Builder interface {
	generate(info Info) kyverno.PolicyViolationTemplate
	build(policy, kind, namespace, name string, rules []kyverno.ViolatedRule) *kyverno.PolicyViolationTemplate
}

type pvBuilder struct{}

func newPvBuilder() *pvBuilder {
	return &pvBuilder{}
}

func (pvb *pvBuilder) generate(info Info) kyverno.PolicyViolationTemplate {
	pv := pvb.build(info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName(), info.Rules)
	return *pv
}

func (pvb *pvBuilder) build(policy, kind, namespace, name string, rules []kyverno.ViolatedRule) *kyverno.PolicyViolationTemplate {
	pv := &kyverno.PolicyViolationTemplate{
		Spec: kyverno.PolicyViolationSpec{
			Policy: policy,
			ResourceSpec: kyverno.ResourceSpec{
				Kind:      kind,
				Name:      name,
				Namespace: namespace,
			},
			ViolatedRules: rules,
		},
	}
	labelMap := map[string]string{
		"policy":   pv.Spec.Policy,
		"resource": pv.Spec.ToKey(),
	}
	pv.SetLabels(labelMap)
	if namespace != "" {
		pv.SetNamespace(namespace)
	}
	pv.SetGenerateName(fmt.Sprintf("%s-", policy))
	return pv
}

func buildPVInfo(er response.EngineResponse) Info {
	info := Info{
		PolicyName: er.PolicyResponse.Policy,
		Resource:   er.PatchedResource,
		Rules:      buildViolatedRules(er),
	}
	return info
}

func buildViolatedRules(er response.EngineResponse) []kyverno.ViolatedRule {
	var violatedRules []kyverno.ViolatedRule
	for _, rule := range er.PolicyResponse.Rules {
		if rule.Success {
			continue
		}
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}
