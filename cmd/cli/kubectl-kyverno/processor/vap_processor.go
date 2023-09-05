package processor

import (
	valuesapi "github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/apis/values"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
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
	Client                    dclient.Interface
	AuditWarn                 bool
	Subresources              []valuesapi.Subresource
}

func (p *ValidatingAdmissionPolicyProcessor) ApplyPolicyOnResource() ([]engineapi.EngineResponse, error) {
	engineResp := validatingadmissionpolicy.Validate(p.ValidatingAdmissionPolicy, *p.Resource)
	ruleResp := engineResp.PolicyResponse.Rules[0]
	if ruleResp.Status() == engineapi.RuleStatusPass {
		p.Rc.Pass++
	} else if ruleResp.Status() == engineapi.RuleStatusFail {
		p.Rc.Fail++
	} else if ruleResp.Status() == engineapi.RuleStatusError {
		p.Rc.Error++
	}
	return []engineapi.EngineResponse{engineResp}, nil
}
