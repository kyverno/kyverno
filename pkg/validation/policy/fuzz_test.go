package policy

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
)

func FuzzValidatePolicy(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		p := &kyverno.ClusterPolicy{}
		ff.GenerateStruct(p)

		Validate(p, nil, nil, nil, true, "admin")
	})
}
