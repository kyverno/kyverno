package loader

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"
	"sigs.k8s.io/kubectl-validate/pkg/validator"
)

type Loader interface {
	Load([]byte) (schema.GroupVersionKind, unstructured.Unstructured, error)
}

type loader struct {
	validator *validator.Validator
}

func New(client openapi.Client) (Loader, error) {
	factory, err := validator.New(client)
	if err != nil {
		return nil, err
	}
	return &loader{
		validator: factory,
	}, nil
}

func (l *loader) Load(document []byte) (schema.GroupVersionKind, unstructured.Unstructured, error) {
	gvk, result, err := l.validator.Parse(document)
	if err != nil {
		return gvk, unstructured.Unstructured{}, fmt.Errorf("failed to parse document (%w)", err)
	}
	if err := l.validator.Validate(result); err != nil {
		return gvk, unstructured.Unstructured{}, fmt.Errorf("failed to validate resource (%w)", err)
	}
	return gvk, *result, nil
}
