package api

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	"github.com/kyverno/kyverno/pkg/registryclient"
)

type Engine interface {
	// Validate applies validation rules from policy on the resource
	Validate(
		ctx context.Context,
		policyContext PolicyContext,
	) *EngineResponse

	// Mutate performs mutation. Overlay first and then mutation patches
	Mutate(
		ctx context.Context,
		policyContext PolicyContext,
	) *EngineResponse

	// VerifyAndPatchImages ...
	VerifyAndPatchImages(
		ctx context.Context,
		rclient registryclient.Client,
		policyContext PolicyContext,
	) (*EngineResponse, *ImageVerificationMetadata)

	// ApplyBackgroundChecks checks for validity of generate and mutateExisting rules on the resource
	// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
	//   - the caller has to check the ruleResponse to determine whether the path exist
	//
	// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
	ApplyBackgroundChecks(
		policyContext PolicyContext,
	) *EngineResponse

	// GenerateResponse checks for validity of generate rule on the resource
	GenerateResponse(
		policyContext PolicyContext,
		gr kyvernov1beta1.UpdateRequest,
	) *EngineResponse

	ContextLoader(
		policyContext PolicyContext,
		ruleName string,
	) ContextLoader
}
