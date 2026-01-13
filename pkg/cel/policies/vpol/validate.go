package vpol

import (
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(vpol v1beta1.ValidatingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	spec := vpol.GetValidatingPolicySpec()

	if spec == nil {
		err = append(err, field.Required(field.NewPath("spec"), "spec must not be nil"))
		warnings := make([]string, 0)
		for _, e := range err.ToAggregate().Errors() {
			warnings = append(warnings, e.Error())
		}
		return warnings, err.ToAggregate()
	}

	compiler := compiler.NewCompiler()
	_, errList := compiler.Compile(vpol, nil)
	if errList != nil {
		err = errList
	}

	if spec.MatchConstraints == nil || len(spec.MatchConstraints.ResourceRules) == 0 {
		err = append(err, field.Required(field.NewPath("spec").Child("matchConstraints"), "a matchConstraints with at least one resource rule is required"))
	}

	if len(err) == 0 {
		return nil, nil
	}

	for _, e := range err.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}

	return warnings, err.ToAggregate()
}
