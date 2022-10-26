package openapi

import (
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func NewFake() ValidateInterface {
	return &fakeValidation{}
}

type fakeValidation struct{}

func (f *fakeValidation) ValidateResource(resource unstructured.Unstructured, apiVersion, kind string) error {
	return nil
}

func (f *fakeValidation) ValidatePolicyMutation(kyvernov1.PolicyInterface) error {
	return nil
}
