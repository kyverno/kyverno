package exception

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
	errVarsNotAllowed   = "variables are currently not allowed in policy exceptions"
)

type ValidationOptions struct {
	Enabled   bool
	Namespace string
}

// Validate checks policy exception is valid
func Validate(ctx context.Context, logger logr.Logger, polex *kyvernov2alpha1.PolicyException, opts ValidationOptions) ([]string, error) {
	var warnings []string
	if !opts.Enabled {
		warnings = append(warnings, disabledPolex)
	} else if opts.Namespace != "" && opts.Namespace != polex.Namespace {
		warnings = append(warnings, namespacesDontMatch)
	}
	var errors []error
	err := validateVariables(polex)
	errs := polex.Validate()
	aggregate := errs.ToAggregate()
	if aggregate != nil {
		errors = append(errors, aggregate.Errors()...)
		errors = append(errors, err)
		return warnings, utilerrors.NewAggregate(errors)
	}
	errors = append(errors, err)
	return warnings, utilerrors.NewAggregate(errors)
}

func validateVariables(polex *kyvernov2alpha1.PolicyException) error {
	vars := hasVariables(polex)
	if len(vars) != 0 {
		return errors.New(errVarsNotAllowed)
	}
	return nil
}

// hasVariables - check for variables in the policy exception
func hasVariables(polex *kyvernov2alpha1.PolicyException) [][]string {
	policyRaw, _ := json.Marshal(polex)
	matches := variables.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}
