package pull

import (
	"io"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/authn/github"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"github.com/kyverno/sdk/extensions/regcreds"
)

func NewKeychain() authn.Keychain {
	return authn.NewMultiKeychain(
		authn.DefaultKeychain,
		github.Keychain,
		authn.NewKeychainFromHelper(ecr.NewECRHelper(ecr.WithLogger(io.Discard))),
		google.Keychain,
		regcreds.AzureKeychain,
	)
}
