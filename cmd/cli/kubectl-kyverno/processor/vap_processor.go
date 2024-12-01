package processor

import (
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidatingAdmissionPolicyProcessor struct {
	Policies             []admissionregistrationv1beta1.ValidatingAdmissionPolicy
	Bindings             []admissionregistrationv1beta1.ValidatingAdmissionPolicyBinding
	Resource             *unstructured.Unstructured
	NamespaceSelectorMap map[string]map[string]string
	PolicyReport         bool
	Rc                   *ResultCounts
	Client               dclient.Interface
	IsCluster            bool
}

func (p *ValidatingAdmissionPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	responses := make([]engineapi.EngineResponse, 0, len(p.Policies))
	for _, policy := range p.Policies {
		policyData := validatingadmissionpolicy.NewPolicyData(policy)
		for _, binding := range p.Bindings {
			if binding.Spec.PolicyName == policy.Name {
				policyData.AddBinding(binding)
			}
		}
		responses, _ = validatingadmissionpolicy.Validate(policyData, *p.Resource, p.NamespaceSelectorMap, p.Client, p.IsCluster)
		for _, r := range responses {
			p.Rc.addValidatingAdmissionResponse(r)
		}
	}
	return responses, nil
}
