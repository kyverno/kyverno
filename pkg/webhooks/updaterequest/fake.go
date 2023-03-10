package updaterequest

import (
	"context"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
)

func NewFake() Generator {
	return &fakeGenerator{}
}

type fakeGenerator struct{}

func (f *fakeGenerator) Apply(ctx context.Context, gr kyvernov1beta1.UpdateRequestSpec) error {
	return nil
}
