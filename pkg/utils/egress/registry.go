package egress

import (
	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kyverno/kyverno/pkg/logging"
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
		registryclient.WithSecretLister(lister, defaultNamespace),
		registryclient.WithImagePullSecrets(secrets...),
		registryclient.WithCredentialHelpers(credentialHelpers...),
		registryclient.WithAllowInsecureRegistry(allowInsecureRegistry),
		registryclient.WithLogger(logging.GlobalLogger()),
		registryclient.WithTransport(guard.Transport()),
		registryclient.WithKeychainWrapper(guard.GuardKeychain),
	)
}
