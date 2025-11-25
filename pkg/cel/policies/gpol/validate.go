package gpol

import (
	policiesv1beta1 "github.com/kyverno/kyverno/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/cel/policies/gpol/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(gpol policiesv1beta1.GeneratingPolicyLike) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	compiler := compiler.NewCompiler()
	_, errList := compiler.Compile(gpol, nil)
	if errList != nil {
		err = errList
	}

	spec := gpol.GetSpec()
	if spec == nil {
		return nil, field.Required(field.NewPath("spec"), "spec is required")
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
