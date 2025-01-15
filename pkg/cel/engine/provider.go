package engine

import (
	"context"

	"github.com/kyverno/kyverno/pkg/cel/policy"
)

type Provider interface {
	CompiledPolicies(context.Context) ([]policy.CompiledPolicy, error)
}
