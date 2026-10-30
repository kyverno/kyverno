package credentials

import (
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// RemoteOptions preserves the SDK's transport, name and provider options while
// making explicit Secret credentials honor each registry request's context.
// The returned options are safe to retain in a compiled policy: they contain
// neither request contexts nor resolved credentials.
//
// This is the only path a policy with its own spec.credentials takes, so it is
// also where the egress guard is applied. regcreds.RemoteOptsFromIvpolCredentials
// pins regcreds.DefaultTransport, which would let such a policy fetch through an
// unguarded transport while every other policy goes through the global client;
// building the options here keeps the same credential composition and adds the
// guard. With no Guard installed these are the SDK's native options, which is
// what the CLI and unit tests expect.
func RemoteOptions(lister corev1listers.SecretLister, creds policiesv1beta1.Credentials, namespace string, logger logr.Logger) ([]remote.Option, []name.Option) {
	providers := make([]string, len(creds.Providers))
	for i, provider := range creds.Providers {
		providers[i] = string(provider)
	}
	guard := egress.Global()
	options, names := registryclient.NewRemoteOptions(
		registryclient.WithSecretLister(lister, namespace),
		registryclient.WithImagePullSecrets(creds.Secrets...),
		registryclient.WithCredentialHelpers(providers...),
		registryclient.WithAllowInsecureRegistry(creds.AllowInsecureRegistry),
		registryclient.WithLogger(logger),
		registryclient.WithTransport(guard.Transport()),
		registryclient.WithKeychainWrapper(guard.GuardKeychain),
	)
	if len(creds.Secrets) == 0 {
		return options, names
	}
	chains := make([]authn.Keychain, 0, 1+len(providers))
	chains = append(chains, NewSecretsKeychain(lister, namespace, logger, creds.Secrets...))
	chains = append(chains, regcreds.KeychainsForProviders(providers...)...)
	// This replaces the keychain composed above rather than layering on it, so it
	// must be guarded here too: the wrapper passed to the SDK only ever reached the
	// chain it built. GuardKeychain is a no-op when no Guard is installed.
	options = append(options, remote.WithAuthFromKeychain(guard.GuardKeychain(authn.NewMultiKeychain(chains...))))
	return options, names
}
