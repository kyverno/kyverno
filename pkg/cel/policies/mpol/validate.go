package mpol

import (
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
	mpolcompiler "github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"github.com/kyverno/kyverno/pkg/toggle"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(mpol v1beta1.MutatingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	spec := mpol.GetSpec()

	if spec == nil {
		err = append(err, field.Required(field.NewPath("spec"), "spec must not be nil"))
		warnings := make([]string, 0, len(err.ToAggregate().Errors()))
		for _, e := range err.ToAggregate().Errors() {
			warnings = append(warnings, e.Error())
		}
		return warnings, err.ToAggregate()
	}

	c := mpolcompiler.NewCompiler()
	_, errList := c.Compile(mpol, nil)
	if len(errList) > 0 {
		err = errList
	}

	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == 0 {
		err = append(err, field.Required(field.NewPath("spec").Child("matchConstraints"), "a matchConstraints with at least one resource rule is required"))
	}

	if mpol.GetNamespace() != "" && !toggle.AllowHTTPInNamespacedPolicies.Enabled() {
		if compiler.ExpressionsUseHTTP(mpolExpressions(spec)...) {
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

func mpolExpressions(spec *v1beta1.MutatingPolicySpec) []string {
	var exprs []string
	for _, v := range spec.Variables {
		exprs = append(exprs, v.Expression)
	}
	for _, mc := range spec.MatchConditions {
		exprs = append(exprs, mc.Expression)
	}
	for _, m := range spec.Mutations {
		if m.ApplyConfiguration != nil {
			exprs = append(exprs, m.ApplyConfiguration.Expression)
		}
		if m.JSONPatch != nil {
			exprs = append(exprs, m.JSONPatch.Expression)
		}
	}
	return exprs
}
