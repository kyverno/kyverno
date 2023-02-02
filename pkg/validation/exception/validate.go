package exception

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/go-logr/logr"
	kyvernov2alpha1 "github.com/kyverno/kyverno/api/kyverno/v2alpha1"
	"github.com/kyverno/kyverno/pkg/engine/variables"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

const (
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace."
	disabledPolex       = "PolicyException resources would not be processed until it is enabled."
)

var forbidden = []*regexp.Regexp{
	regexp.MustCompile(`[^\.](serviceAccountName)\b`),
	regexp.MustCompile(`[^\.](serviceAccountNamespace)\b`),
	regexp.MustCompile(`[^\.](request.userInfo)\b`),
	regexp.MustCompile(`[^\.](request.roles)\b`),
	regexp.MustCompile(`[^\.](request.clusterRoles)\b`),
}

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
	err := validateVariables(polex)
	errs := polex.Validate()
	return warnings, utilerrors.NewAggregate(append(errs.ToAggregate().Errors(), err))
}

func validateVariables(polex *kyvernov2alpha1.PolicyException) error {
	vars := hasVariables(polex)
	if polex.Spec.BackgroundProcessingEnabled() {
		if err := containsUserVariables(polex, vars); err != nil {
			return fmt.Errorf("only select variables are allowed in background mode. Set spec.background=false to disable background mode for this policy rule: %s ", err)
		}
	}
	return nil
}

// hasVariables - check for variables in the policy
func hasVariables(polex *kyvernov2alpha1.PolicyException) [][]string {
	policyRaw, _ := json.Marshal(polex)
	matches := variables.RegexVariables.FindAllStringSubmatch(string(policyRaw), -1)
	return matches
}

// ContainsUserVariables returns error if variable that does not start from request.object
func containsUserVariables(polex *kyvernov2alpha1.PolicyException, vars [][]string) error {
	for _, v := range vars {
		for _, notAllowed := range forbidden {
			if notAllowed.Match([]byte(v[2])) {
				return fmt.Errorf("variable %s is not allowed", v[2])
			}
		}
	}
	return nil
}
