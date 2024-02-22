package processor

import (
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidatingAdmissionPolicyProcessor struct {
	Policies             []v1alpha1.ValidatingAdmissionPolicy
	Bindings             []v1alpha1.ValidatingAdmissionPolicyBinding
	Resource             *unstructured.Unstructured
	NamespaceSelectorMap map[string]map[string]string
	PolicyReport         bool
	Rc                   *ResultCounts
	Client               dclient.Interface
}

func (p *ValidatingAdmissionPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	var responses []engineapi.EngineResponse
	for _, policy := range p.Policies {
		policyData := validatingadmissionpolicy.NewPolicyData(policy)
		for _, binding := range p.Bindings {
			if binding.Spec.PolicyName == policy.Name {
				policyData.AddBinding(binding)
			}
		}
		response, _ := validatingadmissionpolicy.Validate(policyData, *p.Resource, p.NamespaceSelectorMap, p.Client)
		responses = append(responses, response)
		p.Rc.addValidatingAdmissionResponse(policy, response)
	}
	return responses, nil
}
