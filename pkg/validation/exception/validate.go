package exception

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
)

// Validate checks policy exception is valid
func Validate(ctx context.Context, logger logr.Logger, polex *kyvernov2alpha1.PolicyException,
	polexIsEnabled bool, polexNamespace string) ([]string, error) {
	var warnings []string
	if !polexIsEnabled {
		warnings = append(warnings, disabledPolex)
	} else if polexNamespace != "" && polexNamespace != polex.Namespace {
		warnings = append(warnings, namespacesDontMatch)
	}
	errs := polex.Validate()
	return warnings, errs.ToAggregate()
}
