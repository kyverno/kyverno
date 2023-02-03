package exception

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/utils/common"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
	errVarsNotAllowed   = "policy exception \"%s\" should not have variables in match section"
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
	if err := objectHasVariables(polex); err != nil {
		return fmt.Errorf(errVarsNotAllowed, polex.Name)
	}
	return nil

}

func objectHasVariables(object interface{}) error {
	var err error
	objectJSON, err := json.Marshal(object)
	if err != nil {
		return err
	}

	if len(common.RegexVariables.FindAllStringSubmatch(string(objectJSON), -1)) > 0 {
		return fmt.Errorf("variables are not allowed")
	}

	return nil
}
