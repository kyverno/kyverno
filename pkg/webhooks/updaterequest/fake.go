package updaterequest

import (
	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
	admissionv1 "k8s.io/api/admission/v1"
)

func NewFake() Generator {
	return &fakeGenerator{}
}

type fakeGenerator struct{}

func (f *fakeGenerator) Apply(gr kyvernov1beta1.UpdateRequestSpec, action admissionv1.Operation) error {
	return nil
}
