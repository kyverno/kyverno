package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/engine/response"
)

// IsResponseSuccessful return true if all responses are successful
func IsResponseSuccessful(engineReponses []*response.EngineResponse) bool {
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			return false
		}
	}
	return true
}

// BlockRequest returns true when:
// 1. a policy fails (i.e. creates a violation) and validationFailureAction is set to 'enforce'
// 2. a policy has a processing error and failurePolicy is set to 'Fail`
func BlockRequest(er *response.EngineResponse, failurePolicy kyvernov1.FailurePolicyType) bool {
	if er.IsFailed() && er.GetValidationFailureAction() == kyvernov1.Enforce {
		return true
	}
	if er.IsError() && failurePolicy == kyvernov1.Fail {
		return true
	}
	return false
}
