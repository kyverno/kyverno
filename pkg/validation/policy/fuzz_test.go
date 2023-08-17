package policy

import (
	"testing"

	"github.com/go-logr/logr"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/openapi"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
)

var fuzzOpenApiManager openapi.Manager

func init() {
	var err error
	fuzzOpenApiManager, err = openapi.NewManager(logr.Discard())
	if err != nil {
		panic(err)
	}
}

func FuzzValidatePolicy(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		p := &kyverno.ClusterPolicy{}
		ff.GenerateStruct(p)

		Validate(p, nil, nil, true, fuzzOpenApiManager, "admin")
	})
}
