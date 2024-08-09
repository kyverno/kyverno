package generation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/engine"
	admissionv1 "k8s.io/api/admission/v1"
)

func buildURSpecNew(requestType kyvernov2.RequestType, policyKey string, rules []kyvernov1.Rule, trigger kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov2.UpdateRequestSpec {
	ruleCtx := make([]kyvernov2.RuleContext, 0)
	for _, rule := range rules {
		ctx := buildRuleContext(rule, trigger, deleteDownstream)
		ruleCtx = append(ruleCtx, ctx)
	}
	return kyvernov2.UpdateRequestSpec{
		Type:        requestType,
		Policy:      policyKey,
		RuleContext: ruleCtx,
	}
}

func buildRuleContext(rule kyvernov1.Rule, trigger kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov2.RuleContext {
	return kyvernov2.RuleContext{
		Rule:             rule.Name,
		Trigger:          trigger,
		DeleteDownstream: deleteDownstream,
	}
}

func buildURContext(request admissionv1.AdmissionRequest, policyContext *engine.PolicyContext) kyvernov2.UpdateRequestSpecContext {
	return kyvernov2.UpdateRequestSpecContext{
		UserRequestInfo: policyContext.AdmissionInfo(),
		AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
			AdmissionRequest: &request,
			Operation:        request.Operation,
		},
	}
}
