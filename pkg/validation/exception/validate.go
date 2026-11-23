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
	namespacesDontMatch = "PolicyException resource namespace must match the defined namespace; this exception is ignored."
	DisabledPolex       = "PolicyException resources would not be processed until it is enabled."
	polexNamespaceFlag  = "The exceptionNamespace flag is not set: this exception is ignored until --exceptionNamespace is set."
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

// ValidateScopeReach warns when the operator's exceptionNamespace is "*" and polex references a
// Namespaced* policy kind: under "*" there is no central namespace, so the exception only applies
// to a policy of that name in polex's own namespace, not to a same-named policy elsewhere. It also
// warns about spec.namespaces entries other than polex's own namespace, which have no effect.
func ValidateScopeReach(polex *policiesv1beta1.PolicyException, opts ValidationOptions) []string {
	if !opts.Enabled || opts.Namespace != "*" {
		return nil
	}
	var warnings []string
	for _, ns := range polex.Spec.Namespaces {
		if ns != polex.GetNamespace() {
			warnings = append(warnings, fmt.Sprintf(
				"exceptionNamespace is \"*\": spec.namespaces entry %s has no effect, this exception only applies in namespace %s.",
				ns, polex.GetNamespace(),
			))
		}
	}
	for _, ref := range polex.Spec.PolicyRefs {
		if strings.HasPrefix(ref.Kind, "Namespaced") {
			warnings = append(warnings, fmt.Sprintf(
				"exceptionNamespace is \"*\": this exception only applies to %s %s in namespace %s, not to a policy of that name in another namespace.",
				ref.Kind, ref.Name, polex.GetNamespace(),
			))
		}
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
		// this only checks the expression compiles at write time and discards the result, so
		// tracing is irrelevant here
		if _, compileErrs := celcompiler.CompileValidation(path.Index(i), env, validation, false); compileErrs != nil {
			errs = append(errs, compileErrs...)
		}
	}
	return errs
}
