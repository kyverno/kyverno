package exception

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	policyvalidation "github.com/kyverno/kyverno/pkg/validation/policy"
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

// ValidateGlobLengths caps the length of every wildcard-bearing string
// field under the exception's match block and its exceptions' ruleNames,
// reusing the same caps pkg/validation/policy applies to
// Policy/ClusterPolicy rules -- exception names and rule names feed the
// same matching (Exception.Contains uses wildcard.Match on ruleNames) and
// are wildcard-bearing, so they get the same write-time hygiene.
func ValidateGlobLengths(polex *kyvernov2.PolicyException) error {
	match := polex.Spec.Match

	checkFilter := func(prefix string, rf kyvernov1.ResourceFilter) error {
		if err := policyvalidation.ValidateResourceDescriptionGlobLengths(prefix, rf.ResourceDescription); err != nil {
			return err
		}
		return policyvalidation.ValidateUserInfoGlobLengths(prefix, rf.UserInfo)
	}

	for i, rmr := range match.Any {
		if err := checkFilter(fmt.Sprintf("match.any[%d]", i), rmr); err != nil {
			return err
		}
	}
	for i, rmr := range match.All {
		if err := checkFilter(fmt.Sprintf("match.all[%d]", i), rmr); err != nil {
			return err
		}
	}

	for i, e := range polex.Spec.Exceptions {
		if err := policyvalidation.ValidateStringGlobLengths(fmt.Sprintf("exceptions[%d].ruleNames", i), e.RuleNames); err != nil {
			return err
		}
	}

	return nil
}
