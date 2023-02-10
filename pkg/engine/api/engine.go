package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

type EngineContextLoader = func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error

// Engine is the main interface to run policies against resources
type Engine interface {
	// Validate applies validation rules from policy on the resource
	Validate(
		ctx context.Context,
		policyContext PolicyContext,
	) EngineResponse

	// Mutate performs mutation. Overlay first and then mutation patches
	Mutate(
		ctx context.Context,
		policyContext PolicyContext,
	) EngineResponse

	// VerifyAndPatchImages ...
	VerifyAndPatchImages(
		ctx context.Context,
		policyContext PolicyContext,
	) (EngineResponse, ImageVerificationMetadata)

	// ApplyBackgroundChecks checks for validity of generate and mutateExisting rules on the resource
	// 1. validate variables to be substitute in the general ruleInfo (match,exclude,condition)
	//   - the caller has to check the ruleResponse to determine whether the path exist
	//
	// 2. returns the list of rules that are applicable on this policy and resource, if 1 succeed
	ApplyBackgroundChecks(
		ctx context.Context,
		policyContext PolicyContext,
	) EngineResponse

	// GenerateResponse checks for validity of generate rule on the resource
	GenerateResponse(
		ctx context.Context,
		policyContext PolicyContext,
		gr kyvernov1beta1.UpdateRequest,
	) EngineResponse

	ContextLoader(
		policy kyvernov1.PolicyInterface,
		rule kyvernov1.Rule,
	) EngineContextLoader
}
