package egress

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/regcreds"
	"github.com/kyverno/sdk/extensions/registryclient"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// GuardedRemoteOptions builds registry options for credentials declared on a policy.
//
// regcreds.RemoteOptsFromIvpolCredentials pins regcreds.DefaultTransport, so a policy that
// declares its own credentials would otherwise fetch through an unguarded transport while
// every other policy goes through the global client. Building the options here keeps the
// same credential composition and adds the guard.
//
// With no Guard installed this returns the SDK's native options, which is what the CLI and
// unit tests expect.
func GuardedRemoteOptions(
	lister corev1listers.SecretLister,
	defaultNamespace string,
	secrets []string,
	credentialHelpers []string,
	allowInsecureRegistry bool,
) ([]gcrremote.Option, []name.Option) {
	guard := Global()
	return registryclient.NewRemoteOptions(
		registryclient.WithKeychain(requiredSecretsKeychain(lister, defaultNamespace, secrets)),
		registryclient.WithSecretLister(lister, defaultNamespace),
		registryclient.WithImagePullSecrets(secrets...),
		registryclient.WithCredentialHelpers(credentialHelpers...),
		registryclient.WithAllowInsecureRegistry(allowInsecureRegistry),
		registryclient.WithLogger(logging.GlobalLogger()),
		registryclient.WithTransport(guard.Transport()),
		registryclient.WithKeychainWrapper(guard.GuardKeychain),
	)
}

// requiredSecretsKeychain preserves the error a policy used to get when it declared Secrets
// with no lister to read them from.
//
// registryclient.New only builds the Secret keychain when a lister is present, so without
// this the credential chain would silently fall back to the default keychain and fetch
// unauthenticated. regcreds.RemoteOptsFromIvpolCredentials built the keychain regardless and
// failed at resolution with "secret lister is not configured"; keep that. Reachable from the
// CLI, which has no cluster client to build a lister from.
//
// Returns nil when there is nothing to enforce, and registryclient.WithKeychain ignores nil.
func requiredSecretsKeychain(lister corev1listers.SecretLister, defaultNamespace string, secrets []string) authn.Keychain {
	if lister != nil || len(secrets) == 0 {
		return nil
	}
	return regcreds.NewSecretsKeychain(nil, defaultNamespace, logging.GlobalLogger(), secrets...)
}
