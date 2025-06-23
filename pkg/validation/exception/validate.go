package exception

import (
	"context"

	"github.com/go-logr/logr"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	DisabledPolex       = "PolicyException resources would not be processed until it is enabled."
	polexNamespaceFlag  = "The exceptionNamespace flag is not set"
)

type ValidationOptions struct {
	Enabled   bool
	Namespace string
}

// Validate checks policy exception is valid
func ValidateNamespace(ctx context.Context, logger logr.Logger, polexNs string, opts ValidationOptions) []string {
	var warnings []string
	if !opts.Enabled {
		warnings = append(warnings, DisabledPolex)
	} else if opts.Namespace == "" {
		warnings = append(warnings, polexNamespaceFlag)
	} else if opts.Namespace != "*" && opts.Namespace != polexNs {
		warnings = append(warnings, namespacesDontMatch)
	}
	return warnings
}
