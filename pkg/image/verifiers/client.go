package verifiers

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Client interface {
	Options(context.Context) ([]remote.Option, []name.Option, error)
	NameOptions() []name.Option
}
