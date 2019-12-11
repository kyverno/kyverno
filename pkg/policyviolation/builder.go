package policyviolation

import (
	"fmt"

	kyverno "github.com/nirmata/kyverno/pkg/api/kyverno/v1"
	client "github.com/nirmata/kyverno/pkg/dclient"
)

// Builder builds Policy Violation struct
// this is base type of namespaced and cluster policy violation
type Builder interface {
	generate(info Info) []kyverno.PolicyViolation
	build(policy, kind, namespace, name string, rules []kyverno.ViolatedRule) *kyverno.PolicyViolation
}

type pvBuilder struct {
	// dynamic client
	dclient *client.Client
}

func newPvBuilder(dclient *client.Client) *pvBuilder {
	pvb := pvBuilder{
		dclient: dclient,
	}
	return &pvb
}
func (pvb *pvBuilder) generate(info Info) []kyverno.PolicyViolation {
	var owners []kyverno.ResourceSpec
	// get the owners if the resource is blocked or
	// TODO:  https://github.com/nirmata/kyverno/issues/535
	if info.Blocked {
		// get resource owners
		owners = GetOwners(pvb.dclient, info.Resource)
	}
	pvs := pvb.buildPolicyViolations(owners, info)
	return pvs
}

func (pvb *pvBuilder) buildPolicyViolations(owners []kyverno.ResourceSpec, info Info) []kyverno.PolicyViolation {
	var pvs []kyverno.PolicyViolation
	if len(owners) != 0 {
		// there are resource owners
		// generate PV on them
		for _, resource := range owners {
			pv := pvb.build(info.PolicyName, resource.Kind, resource.Namespace, resource.Name, info.Rules)
			pvs = append(pvs, *pv)
		}
	} else {
		// generate PV on resource
		pv := pvb.build(info.PolicyName, info.Resource.GetKind(), info.Resource.GetNamespace(), info.Resource.GetName(), info.Rules)
		pvs = append(pvs, *pv)
	}
	return pvs
}

func (pvb *pvBuilder) build(policy, kind, namespace, name string, rules []kyverno.ViolatedRule) *kyverno.PolicyViolation {
	pv := &kyverno.PolicyViolation{
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
