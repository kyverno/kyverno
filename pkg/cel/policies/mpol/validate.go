package mpol

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/mpol/compiler"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func Validate(mpol *v1alpha1.MutatingPolicy) ([]string, error) {
	warnings := make([]string, 0)
	err := make(field.ErrorList, 0)

	compiler := compiler.NewCompiler()
	_, errList := compiler.Compile(mpol, nil)
	if errList != nil {
		err = errList
	}

	if mpol.Spec.MatchConstraints == nil || len(mpol.Spec.MatchConstraints.ResourceRules) == 0 {
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
