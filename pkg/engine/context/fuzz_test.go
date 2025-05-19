package context

import (
	"testing"

	admissionv1 "k8s.io/api/admission/v1"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
)

var (
	fuzzJp = jmespath.New(config.NewDefaultConfiguration(false))
)

func FuzzHasChanged(f *testing.F) {
	f.Fuzz(func(t *testing.T, obj1, obj2, jString string) {
		ctx := createFuzzContext(obj1, obj2)
		ctx.HasChanged(jString)
	})
}

func createFuzzContext(obj, oldObj string) Interface {
	request := admissionv1.AdmissionRequest{}
	request.Operation = "UPDATE"
	request.Object.Raw = []byte(obj)
	request.OldObject.Raw = []byte(oldObj)

	ctx := NewContext(fuzzJp)
	ctx.AddRequest(request)
	return ctx
}
