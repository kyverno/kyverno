package policyviolation

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	"github.com/nirmata/kyverno/pkg/engine/response"
)

//GeneratePVsFromEngineResponse generate Violations from engine responses
func GeneratePVsFromEngineResponse(ers []response.EngineResponse, log logr.Logger) (pvInfos []Info) {
	for _, er := range ers {
		// ignore creation of PV for resources that are yet to be assigned a name
		if er.PolicyResponse.Resource.Name == "" {
			log.V(4).Info("resource does no have a name assigned yet, not creating a policy violation", "resource", er.PolicyResponse.Resource)
			continue
		}
		// skip when response succeed
		if os.Getenv("POLICY-TYPE") != "POLICYREPORT" {
			if er.IsSuccessful() {
				continue
			}
		}

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
		if os.Getenv("POLICY-TYPE") != "POLICYREPORT" {
			if rule.Success {
				continue
			}
		}
		vrule := kyverno.ViolatedRule{
			Name:    rule.Name,
			Type:    rule.Type,
			Message: rule.Message,
		}
		vrule.Check = "Fail"
		if rule.Success {
			vrule.Check = "Pass"
		}
		violatedRules = append(violatedRules, vrule)
	}
	return violatedRules
}
