package engine

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
)

// IsResponseSuccessful return true if all responses are successful
func IsResponseSuccessful(engineReponses []engineapi.EngineResponse) bool {
	for _, er := range engineReponses {
		if !er.IsSuccessful() {
			return false
		}
	}
	return true
}

// BlockRequest returns true when:
// 2. a policy has a processing error and failurePolicy is set to 'Fail'
// The failure action is evaluated per failing rule so that an Enforce rule blocks the
// request even when the same policy also contains Audit rules, regardless of rule order
// (issue #16557).
func BlockRequest(er engineapi.EngineResponse, failurePolicy kyvernov1.FailurePolicyType) bool {
	if er.IsFailed() && er.EnforcesFailedRules() {
		return true
	}
	if er.IsError() && failurePolicy == kyvernov1.Fail {
		return true
	}
	return false
}
