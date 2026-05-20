package verifiers

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Client interface {
	Keychain() authn.Keychain
	Options(context.Context) ([]remote.Option, error)
	NameOptions() []name.Option
}
