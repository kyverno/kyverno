package exception

import (
	"context"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/webhooks"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
)

// Validate checks policy exception is valid
func Validate(ctx context.Context, logger logr.Logger, polex *kyvernov2alpha1.PolicyException, po *webhooks.ExceptionOptions) (error, []string) {
	var warnings []string
	if !po.EnablePolicyException {
		warnings = append(warnings, disabledPolex)
	} else if po.Namespace != "" {
		if po.Namespace != polex.Namespace {
			warnings = append(warnings, namespacesDontMatch)
		}
	}
	errs := polex.Validate()
	return errs.ToAggregate(), warnings
}
