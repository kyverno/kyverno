package generator

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned"
)

type Generator[T any] interface {
	Generate(context.Context, versioned.Interface, T, logr.Logger) (T, error)
}
