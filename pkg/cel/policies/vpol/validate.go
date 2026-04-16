package vpol

import (

	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/compiler"
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

func vpolExpressions(spec *v1beta1.ValidatingPolicySpec) []string {
	var exprs []string
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
