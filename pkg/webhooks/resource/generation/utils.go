package generation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/engine"
	admissionv1 "k8s.io/api/admission/v1"
)

func buildURSpec(requestType kyvernov2.RequestType, policyKey, ruleName string, resource kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov2.UpdateRequestSpec {
	return kyvernov2.UpdateRequestSpec{
		Type:             requestType,
		Policy:           policyKey,
		Rule:             ruleName,
		Resource:         resource,
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
