package vpol

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

func FuzzValidateValidatingPolicy(f *testing.F) {
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		policy := &policiesv1beta1.ValidatingPolicy{}
		_ = fuzz.NewConsumer(data).GenerateStruct(policy)
		_, _ = Validate(policy)
	})
}

func FuzzValidateNamespacedValidatingPolicy(f *testing.F) {
	f.Add([]byte("{}"))
	f.Fuzz(func(t *testing.T, data []byte) {
		policy := &policiesv1beta1.NamespacedValidatingPolicy{}
		_ = fuzz.NewConsumer(data).GenerateStruct(policy)
		_, _ = Validate(policy)
	})
}
