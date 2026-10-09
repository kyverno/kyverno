package credentials

import (
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/sdk/extensions/regcreds"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// RemoteOptions preserves the SDK's transport, name and provider options while
// making explicit Secret credentials honor each registry request's context.
// The returned options are safe to retain in a compiled policy: they contain
// neither request contexts nor resolved credentials.
func RemoteOptions(lister corev1listers.SecretLister, creds policiesv1beta1.Credentials, namespace string, logger logr.Logger) ([]remote.Option, []name.Option) {
	options, names := regcreds.RemoteOptsFromIvpolCredentials(lister, creds, namespace, logger)
	if len(creds.Secrets) == 0 {
		return options, names
	}
	chains := make([]authn.Keychain, 0, 1+len(creds.Providers))
	chains = append(chains, NewSecretsKeychain(lister, namespace, logger, creds.Secrets...))
	providers := make([]string, len(creds.Providers))
	for i, provider := range creds.Providers {
		providers[i] = string(provider)
	}
	chains = append(chains, regcreds.KeychainsForProviders(providers...)...)
	options = append(options, remote.WithAuthFromKeychain(authn.NewMultiKeychain(chains...)))
	return options, names
}
