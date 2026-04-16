package gpol

import (

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	gpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"github.com/kyverno/kyverno/pkg/toggle"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(gpol v1beta1.GeneratingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	spec := gpol.GetSpec()
	if spec == nil {
		err = append(err, field.Required(field.NewPath("spec"), "spec must not be nil"))
		for _, e := range err.ToAggregate().Errors() {
			warnings = append(warnings, e.Error())
		}
		return warnings, err.ToAggregate()
	}

	c := gpolcompiler.NewCompiler()
	_, errList := c.Compile(gpol, nil)
	if errList != nil {
		err = errList
	}

	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == 0 {
		err = append(err, field.Required(field.NewPath("spec").Child("matchConstraints"), "a matchConstraints with at least one resource rule is required"))
	}

	if gpol.GetNamespace() != "" && !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		if compiler.ExpressionsUseHTTP(gpolExpressions(spec)...) {
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

func gpolExpressions(spec *v1beta1.GeneratingPolicySpec) []string {
	var exprs []string
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	for _, mc := range spec.MatchConditions {
		exprs = append(exprs, mc.Expression)
	}
	for _, g := range spec.Generation {
		exprs = append(exprs, g.Expression)
	}
	return exprs
}
