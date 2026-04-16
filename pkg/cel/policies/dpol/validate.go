package dpol

import (

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	dpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/dpol/compiler"
	"github.com/kyverno/kyverno/pkg/toggle"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(dpol v1beta1.DeletingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	spec := dpol.GetDeletingPolicySpec()
	if spec == nil {
		err = append(err, field.Required(field.NewPath("spec"), "spec must not be nil"))
		for _, e := range err.ToAggregate().Errors() {
			warnings = append(warnings, e.Error())
		}
		return warnings, err.ToAggregate()
	}

	c := dpolcompiler.NewCompiler()
	_, errList := c.Compile(dpol, nil)
	if errList != nil {
		err = errList
	}

	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == 0 {
		err = append(err, field.Required(field.NewPath("spec").Child("matchConstraints"), "a matchConstraints with at least one resource rule is required"))
	}

	if dpol.GetNamespace() != "" && !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		if compiler.ExpressionsUseHTTP(dpolExpressions(spec)...) {
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

func dpolExpressions(spec *v1beta1.DeletingPolicySpec) []string {
	var exprs []string
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	for _, c := range spec.Conditions {
		exprs = append(exprs, c.Expression)
	}
	return exprs
}
