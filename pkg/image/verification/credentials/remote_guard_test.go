package credentials

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// RemoteOptions is the only path a policy with its own spec.credentials takes, so it is the
// one place the egress guard has to be wired. regcreds.RemoteOptsFromIvpolCredentials pins
// the SDK's DefaultTransport, so a regression here would let such a policy fetch unguarded
// while every other policy goes through the global client.
func TestRemoteOptionsCarryTheEgressGuard(t *testing.T) {
	// A literal metadata address needs no DNS and no network, so the guard rejects it before
	// the first byte is written. Exercising the options beats counting them: a non-empty
	// slice says nothing about whether the guard reached them.
	//
	// Both cases below go through the same transport, which is the whole point: credential
	// composition differs between them, but the destination check does not depend on which
	// keychain won. transport_test.go pins the transport's own behavior, including the redirect
	// and token-challenge paths that run after credentials have resolved.
	blocked, err := name.ParseReference("169.254.169.254/app:v1")
	require.NoError(t, err)

	// Credentials resolve before the transport dials, so the lister has to be present and
	// working: a nil one fails the declared-Secrets cases with "secret lister is not configured"
	// and the request never reaches the transport, which would pass this test for the wrong
	// reason. A NotFound is tolerated by secretsKeychain, so an empty lister resolves to
	// anonymous and the fetch proceeds to the destination check.
	lister := contextualSecretLister{namespace: &contextualSecretNamespaceLister{
		get: func(context.Context, string) (*corev1.Secret, error) {
			return nil, apierrors.NewNotFound(corev1.Resource("secrets"), "regcred")
		},
	}}

	for _, test := range []struct {
		name  string
		creds policiesv1beta1.Credentials
	}{
		// No declared Secrets: the options carry the SDK-composed keychain and the guarded
		// transport.
		{name: "no secrets"},
		// Declared Secrets take the branch that replaces that keychain outright with a
		// context-aware chain. The transport is unchanged by that swap, so this fetch must
		// still be refused.
		{name: "explicit secrets", creds: policiesv1beta1.Credentials{Secrets: []string{"regcred"}}},
		{name: "secrets and providers", creds: policiesv1beta1.Credentials{
			Secrets:   []string{"regcred"},
			Providers: []policiesv1beta1.CredentialsProvidersType{"default"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetGuard(t)
			require.NoError(t, egress.SetGlobal(egress.Config{Mode: egress.ModeEnforce}))

			opts, _ := RemoteOptions(lister, test.creds, "kyverno", logging.GlobalLogger())
			require.NotEmpty(t, opts)
			_, err := gcrremote.Get(blocked, opts...)
			require.ErrorIs(t, err, egress.ErrAddressBlocked, "the returned options did not carry the guard")
		})
	}
}

// With no Guard installed these must stay the SDK's native options, which is what the CLI
// and unit tests rely on.
func TestRemoteOptionsWithoutGuard(t *testing.T) {
	resetGuard(t)
	egress.RestoreGlobal(nil)

	opts, nameOpts := RemoteOptions(nil, policiesv1beta1.Credentials{}, "kyverno", logging.GlobalLogger())
	require.NotEmpty(t, opts)
	require.Empty(t, nameOpts)

	// allowInsecureRegistry must still reach the SDK, which adds name.Insecure for it.
	_, insecureNameOpts := RemoteOptions(nil, policiesv1beta1.Credentials{AllowInsecureRegistry: true}, "kyverno", logging.GlobalLogger())
	require.Len(t, insecureNameOpts, 1)
}

// A policy that declares Secrets with no lister to read them from must fail loudly rather
// than quietly fetching unauthenticated. Reachable from the CLI, which has no cluster client
// to build a lister from.
func TestRemoteOptionsRequireAListerForDeclaredSecrets(t *testing.T) {
	keychain := NewSecretsKeychain(nil, "kyverno", logging.GlobalLogger(), "regcred")
	_, err := keychain.Resolve(fakeResource("registry.example.com"))
	require.ErrorContains(t, err, "secret lister is not configured")
}

type fakeResource string

func (r fakeResource) String() string      { return string(r) }
func (r fakeResource) RegistryStr() string { return string(r) }

// resetGuard restores whatever Guard was installed, so these tests can run alongside others
// without leaking process-wide state.
func resetGuard(t *testing.T) {
	t.Helper()
	previous := egress.Global()
	t.Cleanup(func() { egress.RestoreGlobal(previous) })
}
