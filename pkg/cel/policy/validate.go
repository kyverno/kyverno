package policy

import "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"

func Validate(vpol *v1alpha1.ValidatingPolicy) ([]string, error) {
	var warnings []string
	compiler := NewCompiler()
	_, err := compiler.CompileValidating(vpol, nil)
	if err != nil {
		return warnings, err.ToAggregate()
	}

	err = vpol.Spec.Validate()
	if err != nil {
		return warnings, err.ToAggregate()
	}
	return nil, nil
}
