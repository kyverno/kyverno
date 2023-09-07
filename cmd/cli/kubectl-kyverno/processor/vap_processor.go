package processor

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
	"k8s.io/api/admissionregistration/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidatingAdmissionPolicyProcessor struct {
	ValidatingAdmissionPolicy v1alpha1.ValidatingAdmissionPolicy
	Resource                  *unstructured.Unstructured
	PolicyReport              bool
	Rc                        *ResultCounts
}

func (p *ValidatingAdmissionPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	engineResp := validatingadmissionpolicy.Validate(p.ValidatingAdmissionPolicy, *p.Resource)
	p.Rc.addValidatingAdmissionResponse(p.ValidatingAdmissionPolicy, engineResp)
	return []engineapi.EngineResponse{engineResp}, nil
}
