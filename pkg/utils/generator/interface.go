package generator

import (
	"context"

	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
)

type Generator[T any] interface {
	Generate(context.Context, versioned.Interface, T) (T, error)
}
