package policy

import "github.com/kyverno/kyverno/api/policies.kyverno.io/v1alpha1"

func Validate(vpol *v1alpha1.ValidatingPolicy) ([]string, error) {
	compiler := NewCompiler()
	_, err := compiler.CompileValidating(vpol, nil)
	if err == nil {
		return nil, nil
	}

	warnings := make([]string, 0, len(err.ToAggregate().Errors()))
	for _, e := range err.ToAggregate().Errors() {
		warnings = append(warnings, e.Error())
	}

	return warnings, err.ToAggregate()
}
