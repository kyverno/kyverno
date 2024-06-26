package updaterequest

import (
	"context"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
)

func NewFake() Generator {
	return &fakeGenerator{}
}

type fakeGenerator struct{}

func (f *fakeGenerator) Apply(ctx context.Context, gr kyvernov2.UpdateRequestSpec) error {
	return nil
}
