package api

import (
	"context"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	enginecontext "github.com/kyverno/kyverno/pkg/engine/context"
)

// EngineContextLoader provides a function to load context entries from the various clients initialised with the engine ones
type EngineContextLoader = func(ctx context.Context, contextEntries []kyvernov1.ContextEntry, jsonContext enginecontext.Interface) error

// EngineContextLoaderFactory provides an EngineContextLoader given a policy and rule name
type EngineContextLoaderFactory = func(policy kyvernov1.PolicyInterface, rule kyvernov1.Rule) EngineContextLoader

// Engine is the main interface to run policies against resources
type Engine interface {
	// Validate applies validation rules from policy on the resource
	Validate(
		ctx context.Context,
		policyContext PolicyContext,
		allowedOperations ...kyvernov1.AdmissionOperation,
	) EngineResponse

	// Mutate performs mutation. Overlay first and then mutation patches
	Mutate(
		ctx context.Context,
		policyContext PolicyContext,
	) EngineResponse

	// Generate checks for validity of generate rule on the resource
	Generate(
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

	ContextLoader(
		policy kyvernov1.PolicyInterface,
		rule kyvernov1.Rule,
	) EngineContextLoader
}
