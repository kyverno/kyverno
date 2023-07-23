package common

import (
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	"github.com/kyverno/kyverno/pkg/validatingadmissionpolicy"
)

type ValidatingAdmissionPolicies struct{}

func (p *ValidatingAdmissionPolicies) ApplyPolicyOnResource(c ApplyPolicyConfig) ([]engineapi.EngineResponse, error) {
	engineResp, err := validatingadmissionpolicy.Validate(c.ValidatingAdmissionPolicy, *c.Resource)
	ruleResp := engineResp.PolicyResponse.Rules[0]
	if err != nil {
		return nil, err
	}
	if ruleResp.Status() == engineapi.RuleStatusPass {
		c.Rc.Pass++
	} else if ruleResp.Status() == engineapi.RuleStatusFail {
		c.Rc.Fail++
	} else if ruleResp.Status() == engineapi.RuleStatusError {
		c.Rc.Error++
	}

	return []engineapi.EngineResponse{*engineResp}, nil
}
