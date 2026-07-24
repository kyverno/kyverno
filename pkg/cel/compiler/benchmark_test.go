package compiler

import (
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

func BenchmarkCompileValidation(b *testing.B) {
	env, err := NewBaseEnv()
	if err != nil {
		b.Fatalf("NewBaseEnv() error = %v", err)
	}

	rule := admissionregistrationv1.Validation{
		Expression:        "'kyverno' in ['kyverno', 'policy']",
		Message:           "team label must be set to kyverno",
		MessageExpression: "'missing or invalid team label'",
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		compiled, errs := CompileValidation(nil, env, rule)
		if len(errs) > 0 {
			b.Fatalf("CompileValidation() errs = %v", errs)
		}
		if compiled.Program == nil {
			b.Fatal("CompileValidation() returned nil program")
		}
	}
}
