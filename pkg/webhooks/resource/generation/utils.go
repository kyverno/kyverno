package generation

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/engine"
	admissionv1 "k8s.io/api/admission/v1"
)

func buildURSpec(requestType kyvernov1beta1.RequestType, policyKey, ruleName string, resource kyvernov1.ResourceSpec, deleteDownstream bool) kyvernov1beta1.UpdateRequestSpec {
	return kyvernov1beta1.UpdateRequestSpec{
		Type:             requestType,
		Policy:           policyKey,
		Rule:             ruleName,
		Resource:         resource,
		DeleteDownstream: deleteDownstream,
	}
}

func buildURContext(request admissionv1.AdmissionRequest, policyContext *engine.PolicyContext) kyvernov1beta1.UpdateRequestSpecContext {
	return kyvernov1beta1.UpdateRequestSpecContext{
		UserRequestInfo: policyContext.AdmissionInfo(),
		AdmissionRequestInfo: kyvernov1beta1.AdmissionRequestInfoObject{
			AdmissionRequest: &request,
			Operation:        request.Operation,
		},
	}
}
