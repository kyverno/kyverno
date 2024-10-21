package exception

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/policy"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
)

type ValidationOptions struct {
	Enabled   bool
	Namespace []string
}

// Validate checks policy exception is valid
func Validate(ctx context.Context, logger logr.Logger, polex *kyvernov2.PolicyException, opts ValidationOptions) ([]string, error) {
	var warnings []string
	if !opts.Enabled {
		warnings = append(warnings, disabledPolex)
	} else if len(opts.Namespace) != 0 && !policy.Contains(opts.Namespace, polex.Namespace) {
		warnings = append(warnings, namespacesDontMatch)
	}
	errs := polex.Validate()
	return warnings, errs.ToAggregate()
}
