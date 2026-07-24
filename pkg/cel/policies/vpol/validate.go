package vpol

import (
	"slices"

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/autogen"
	vpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"github.com/kyverno/kyverno/pkg/toggle"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(vpol v1beta1.ValidatingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	spec := vpol.GetValidatingPolicySpec()

	if spec == nil {
		err = append(err, field.Required(field.NewPath("spec"), "spec must not be nil"))
		warnings := make([]string, 0, len(err.ToAggregate().Errors()))
		for _, e := range err.ToAggregate().Errors() {
			warnings = append(warnings, e.Error())
		}
		return warnings, err.ToAggregate()
	}

	c := vpolcompiler.NewCompiler()
	_, errList := c.Compile(vpol, nil)
	if errList != nil {
		err = errList
	}

	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == 0 {
		err = append(err, field.Required(field.NewPath("spec").Child("matchConstraints"), "a matchConstraints with at least one resource rule is required"))
	}

	if identifierErrs := validateIdentifiers(vpol, spec); identifierErrs != nil {
		err = append(err, identifierErrs...)
	}

	if vpol.GetNamespace() != "" && !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		if compiler.ExpressionsUseHTTP(vpolExpressions(spec)...) {
			err = append(err, field.Forbidden(field.NewPath("spec"), "http.* is not allowed in namespaced policies; set --allowHTTPInNamespacedPolicies to enable"))
		}
	}

	if len(err) == 0 {
		return warnings, nil
	}

	for _, e := range err.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}

	return warnings, err.ToAggregate()
}

// validateIdentifiers parses the optional autogen.IdentifiersAnnotation and
// checks that every identifier it assigns to a validation (matched by
// expression text) is unique within the policy, so autogen rule names don't
// collide. A malformed annotation, or one that references an expression that
// doesn't match any validation in the policy, is reported as an error.
func validateIdentifiers(vpol v1beta1.ValidatingPolicyLike, spec *v1beta1.ValidatingPolicySpec) field.ErrorList {
	annotationPath := field.NewPath("metadata").Child("annotations").Key(autogen.IdentifiersAnnotation)
	byExpression, err := autogen.IdentifiersFromAnnotations(vpol.GetAnnotations())
	if err != nil {
		return field.ErrorList{field.Invalid(annotationPath, vpol.GetAnnotations()[autogen.IdentifiersAnnotation], err.Error())}
	}
	if len(byExpression) == 0 {
		return nil
	}

	var allErrs field.ErrorList
	expressions := make(map[string]bool, len(spec.Validations))
	identifiers := make([]string, len(spec.Validations))
	for i, v := range spec.Validations {
		expressions[v.Expression] = true
		identifiers[i] = byExpression[v.Expression]
	}

	strayExpressions := make([]string, 0, len(byExpression))
	for expression := range byExpression {
		if !expressions[expression] {
			strayExpressions = append(strayExpressions, expression)
		}
	}
	slices.Sort(strayExpressions)
	for _, expression := range strayExpressions {
		allErrs = append(allErrs, field.Invalid(annotationPath, expression, "does not match any validation expression in spec.validations"))
	}

	allErrs = append(allErrs, autogen.ValidateUniqueIdentifiers(field.NewPath("spec").Child("validations"), identifiers)...)
	return allErrs
}

func vpolExpressions(spec *v1beta1.ValidatingPolicySpec) []string {
	exprs := make([]string, 0, len(spec.Variables)+len(spec.MatchConditions)+len(spec.Validations)*2+len(spec.AuditAnnotations))
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	for _, mc := range spec.MatchConditions {
		exprs = append(exprs, mc.Expression)
	}
	for _, val := range spec.Validations {
		exprs = append(exprs, val.Expression, val.MessageExpression)
	}
	for _, aa := range spec.AuditAnnotations {
		exprs = append(exprs, aa.ValueExpression)
	}
	return exprs
}
