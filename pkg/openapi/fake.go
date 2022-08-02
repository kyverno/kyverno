package openapi

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

func NewFake() ValidateInterface {
	return &fakeValidation{}
}

type fakeValidation struct {
}

func (f *fakeValidation) ValidateResource(resource unstructured.Unstructured, apiVersion, kind string) error {
	return nil
}
