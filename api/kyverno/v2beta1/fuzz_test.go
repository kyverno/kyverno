package v2beta1

import (
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func FuzzV2beta1PolicyValidate(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		p := Policy{}
		ff.GenerateStruct(&p)
		_, _ = p.Validate(nil)
	})
}

var (
	path = field.NewPath("dummy")
)

func FuzzV2beta1ImageVerification(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		iv := ImageVerification{}
		ff.GenerateStruct(&iv)
		iv.Validate(false, path)
	})
}

func FuzzV2beta1MatchResources(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		mr := &MatchResources{}
		ff.GenerateStruct(&mr)
		mr.ValidateResourceWithNoUserInfo(path, false, nil)
		mr.Validate(path, false, nil)
	})
}

func FuzzV2beta1ClusterPolicy(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		ff := fuzz.NewConsumer(data)
		cp := &ClusterPolicy{}
		ff.GenerateStruct(&cp)
		cp.HasAutoGenAnnotation()
		cp.HasMutateOrValidateOrGenerate()
		cp.HasMutate()
		cp.HasValidate()
		cp.HasGenerate()
		cp.HasVerifyImages()
		cp.AdmissionProcessingEnabled()
		cp.BackgroundProcessingEnabled()
		cp.Validate(nil)
	})
}
