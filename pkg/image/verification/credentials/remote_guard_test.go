package credentials

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	gcrremote "github.com/google/go-containerregistry/pkg/v1/remote"
	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/utils/egress"
	"github.com/stretchr/testify/require"
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
	// This pins that the options carry a guard, not which one: the transport and the keychain
	// wrapper both reject a literal address, so removing either alone still fails this fetch.
	// They are pinned individually in pkg/utils/egress -- TestGuardKeychainValidatesBeforeResolving
	// for the keychain, and transport_test.go for the transport, including the redirect and
	// token-challenge paths that run after credentials have resolved and only it can catch.
	// What is pinned only here is the explicit-Secrets branch below, which replaces the
	// SDK-composed keychain and so needs its own wrap.
	blocked, err := name.ParseReference("169.254.169.254/app:v1")
	require.NoError(t, err)

	for _, test := range []struct {
		name  string
		creds policiesv1beta1.Credentials
	}{
		// No declared Secrets: the options carry the SDK-composed keychain and the guarded
		// transport.
		{name: "no secrets"},
		// Declared Secrets take the branch that replaces that keychain outright with a
		// context-aware chain. That replacement is applied after the SDK has wrapped its own
		// chain, so it has to be guarded separately or this branch would fetch unguarded.
		{name: "explicit secrets", creds: policiesv1beta1.Credentials{Secrets: []string{"regcred"}}},
		{name: "secrets and providers", creds: policiesv1beta1.Credentials{
			Secrets:   []string{"regcred"},
			Providers: []policiesv1beta1.CredentialsProvidersType{"default"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			resetGuard(t)
			require.NoError(t, egress.SetGlobal(egress.Config{Mode: egress.ModeEnforce}))

			opts, _ := RemoteOptions(nil, test.creds, "kyverno", logging.GlobalLogger())
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
