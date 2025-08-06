package gpol

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	admissionv1 "k8s.io/api/admission/v1"
)

func buildURSpecNew(requestType kyvernov2.RequestType, policyName string, trigger kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov2.UpdateRequestSpec {
	ruleCtx := buildRuleContext(policyName, trigger, deleteDownstream)
	return kyvernov2.UpdateRequestSpec{
		Type:        requestType,
		Policy:      policyName,
		RuleContext: []kyvernov2.RuleContext{ruleCtx},
	}
}

func buildRuleContext(policyName string, trigger kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov2.RuleContext {
	return kyvernov2.RuleContext{
		Rule:             policyName,
		Trigger:          trigger,
		DeleteDownstream: deleteDownstream,
		CacheRestore:     false,
	}
}

func buildURContext(request admissionv1.AdmissionRequest, userInfo kyvernov2.RequestInfo) kyvernov2.UpdateRequestSpecContext {
	return kyvernov2.UpdateRequestSpecContext{
		UserRequestInfo: userInfo,
		AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
			AdmissionRequest: &request,
			Operation:        request.Operation,
		},
	}
}
