package policy

import (
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

func FuzzValidatePolicy(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		p := &kyverno.ClusterPolicy{}
		ff.GenerateStruct(p)

		Validate(p, nil, nil, true, "admin", "admin")
	})
}
