package api

import (
	"context"

	"github.com/kyverno/kyverno/pkg/config"
)

type Engine interface {
	// Validate applies validation rules from policy on the resource
	Validate(
		ctx context.Context,
		contextLoader ContextLoaderFactory,
		policyContext PolicyContext,
		cfg config.Configuration,
	)
}
