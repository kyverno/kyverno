package processor

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidatingAdmissionPolicyProcessor struct {
	Policies     []v1alpha1.ValidatingAdmissionPolicy
	Resource     *unstructured.Unstructured
	PolicyReport bool
	Rc           *ResultCounts
}

func (p *ValidatingAdmissionPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	var responses []engineapi.EngineResponse
	for _, policy := range p.Policies {
		response := validatingadmissionpolicy.Validate(policy, *p.Resource)
		responses = append(responses, response)
		p.Rc.addValidatingAdmissionResponse(policy, response)
	}
	return responses, nil
}
