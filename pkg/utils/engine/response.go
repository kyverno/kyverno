package engine

import (
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
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

// CheckEngineResponse return true if engine response is not successful and validation failure action is set to 'enforce'
func CheckEngineResponse(er *response.EngineResponse) bool {
	return !er.IsSuccessful() && er.GetValidationFailureAction() == kyverno.Enforce
}
