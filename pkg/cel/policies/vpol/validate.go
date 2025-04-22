package vpol

import (
	"github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"
	"github.com/kyverno/kyverno/pkg/cel/policies/vpol/compiler"
)

func Validate(vpol *v1alpha1.ValidatingPolicy) ([]string, error) {
	compiler := compiler.NewCompiler()
	_, err := compiler.Compile(vpol, nil)
	if err == nil {
		return nil, nil
	}
	warnings := make([]string, 0, len(err.ToAggregate().Errors()))
	for _, e := range err.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}
	return warnings, err.ToAggregate()
}
