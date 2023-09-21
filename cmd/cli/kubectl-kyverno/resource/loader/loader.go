package loader

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/openapi"
	"sigs.k8s.io/kubectl-validate/pkg/validator"
)

type Loader interface {
	Load([]byte) (unstructured.Unstructured, error)
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

func (l *loader) Load(document []byte) (unstructured.Unstructured, error) {
	_, result, err := l.validator.Parse(document)
	if err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to parse document (%w)", err)
	}
	// TODO: remove DeepCopy when fixed upstream
	if err := l.validator.Validate(result.DeepCopy()); err != nil {
		return unstructured.Unstructured{}, fmt.Errorf("failed to validate resource (%w)", err)
	}
	return *result, nil
}
