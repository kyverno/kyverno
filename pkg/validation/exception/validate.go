package exception

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	policieskyvernoio "github.com/kyverno/api/api/policies.kyverno.io"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	celcompiler "github.com/kyverno/kyverno/pkg/cel/compiler"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
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

// evaluatesCompensatingControls reports whether the engine behind a policy kind honors an
// exception's spec.validations. Only the ValidatingPolicy engine does today; the mutating,
// generating, deleting and image-validating engines match exceptions and ignore the field.
func evaluatesCompensatingControls(kind string) bool {
	return kind == policieskyvernoio.ValidatingPolicyKind || kind == policieskyvernoio.NamespacedValidatingPolicyKind
}

// ValidateCompensatingControls warns when spec.validations is set on an exception referencing a
// kind whose engine ignores it. The field is shared by every policy kind, so without this it
// reads to the author as if the bypass were guarded when it is not.
func ValidateCompensatingControls(spec *policiesv1beta1.PolicyExceptionSpec) []string {
	if len(spec.Validations) == 0 {
		return nil
	}
	var unsupported []string
	for _, ref := range spec.PolicyRefs {
		if !evaluatesCompensatingControls(ref.Kind) && !slices.Contains(unsupported, ref.Kind) {
			unsupported = append(unsupported, ref.Kind)
		}
	}
	if len(unsupported) == 0 {
		return nil
	}
	return []string{fmt.Sprintf(
		"spec.validations is only evaluated for ValidatingPolicy and NamespacedValidatingPolicy; it will be ignored for the referenced %s.",
		strings.Join(unsupported, ", "),
	)}
}

// ValidateCompensatingControlExpressions rejects a malformed control at write time, before it can
// fail to compile inside the engine and take down every ValidatingPolicy referencing it. Match
// conditions are left alone: each policy kind compiles those against its own environment.
func ValidateCompensatingControlExpressions(polex *policiesv1beta1.PolicyException) field.ErrorList {
	if len(polex.Spec.Validations) == 0 {
		return nil
	}
	if !slices.ContainsFunc(polex.Spec.PolicyRefs, func(ref policiesv1beta1.PolicyRef) bool {
		return evaluatesCompensatingControls(ref.Kind)
	}) {
		return nil
	}
	env, err := vpolcompiler.NewExceptionEnv(polex.GetNamespace())
	if err != nil {
		// failing to build our own environment is not the author's fault, and rejecting the
		// write would be the wrong way to report it; the engine surfaces it on the policy
		return nil
	}
	path := field.NewPath("spec", "validations")
	var errs field.ErrorList
	for i, validation := range polex.Spec.Validations {
		if _, compileErrs := celcompiler.CompileValidation(path.Index(i), env, validation); compileErrs != nil {
			errs = append(errs, compileErrs...)
		}
	}
	return errs
}
