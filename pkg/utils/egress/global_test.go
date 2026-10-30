package egress

import (
	"context"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/require"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// resetGlobal restores whatever Guard was installed, so these tests can run alongside others
// in the package without leaking process-wide state.
func resetGlobal(t *testing.T) {
	t.Helper()
	previous := global.Load()
	t.Cleanup(func() { global.Store(previous) })
}

// registryResource is the authn.Resource a keychain is asked about.
type registryResource string

func (r registryResource) String() string      { return string(r) }
func (r registryResource) RegistryStr() string { return string(r) }

type keychainFunc func(authn.Resource) (authn.Authenticator, error)

func (f keychainFunc) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	return f(resource)
}

func TestGlobalNilGuardLeavesSDKDefaults(t *testing.T) {
	resetGlobal(t)
	global.Store(nil)

	guard := Global()
	require.Nil(t, guard)
	// A nil Guard must stay usable: registryclient.WithTransport(nil) falls back to the SDK
	// default, and the keychain wrapper must hand back the chain it was given.
	require.Nil(t, guard.Transport())
	require.Nil(t, guard.Policy())
	inner := authn.NewMultiKeychain()
	require.Equal(t, inner, guard.GuardKeychain(inner))
}

func TestSetGlobalRejectsInvalidConfig(t *testing.T) {
	resetGlobal(t)
	require.Error(t, SetGlobal(Config{Mode: "disabled"}))
	require.Error(t, SetGlobal(Config{Allowlist: []string{"*.internal"}}))
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce, Allowlist: []string{"10.0.0.0/8"}}))
	require.NotNil(t, Global().Transport())
	require.NotNil(t, Global().Policy())
}

// The guard must validate before credentials are resolved, so a blocked registry never sees
// them whatever keychain is underneath.
func TestGuardKeychainValidatesBeforeResolving(t *testing.T) {
	resetGlobal(t)
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce}))

	resolved := false
	inner := keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
		resolved = true
		return authn.Anonymous, nil
	})
	guarded := Global().GuardKeychain(inner)

	_, err := guarded.Resolve(registryResource("169.254.169.254"))
	require.ErrorIs(t, err, ErrAddressBlocked)
	require.False(t, resolved, "credentials must not be resolved for a blocked registry")
}

func TestGuardKeychainResolvesAllowedRegistry(t *testing.T) {
	resetGlobal(t)
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce, Allowlist: []string{"10.20.30.1"}}))

	resolved := false
	guarded := Global().GuardKeychain(keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
		resolved = true
		return authn.Anonymous, nil
	}))

	_, err := guarded.Resolve(registryResource("10.20.30.1"))
	require.NoError(t, err)
	require.True(t, resolved)
}

func TestGuardKeychainPropagatesCancellation(t *testing.T) {
	resetGlobal(t)
	require.NoError(t, SetGlobal(Config{}))

	guarded := Global().GuardKeychain(keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
		return authn.Anonymous, nil
	}))
	contextual, ok := guarded.(authn.ContextKeychain)
	require.True(t, ok, "the wrapper must forward the request context")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := contextual.ResolveContext(ctx, registryResource("registry.example.com"))
	require.ErrorIs(t, err, context.Canceled)
}

// GuardedRemoteOptions is the path a policy with its own spec.credentials takes. It must not
// fall back to the SDK's pinned DefaultTransport while a Guard is installed.
func TestGuardedRemoteOptions(t *testing.T) {
	resetGlobal(t)
	require.NoError(t, SetGlobal(Config{Mode: ModeEnforce}))

	opts, nameOpts := GuardedRemoteOptions(nil, "kyverno", nil, nil, false)
	require.NotEmpty(t, opts)
	require.Empty(t, nameOpts)

	// allowInsecureRegistry must still reach the SDK, which adds name.Insecure for it.
	_, insecureNameOpts := GuardedRemoteOptions(nil, "kyverno", nil, nil, true)
	require.Len(t, insecureNameOpts, 1)

	// With no Guard installed the same call must still produce usable native options.
	global.Store(nil)
	nativeOpts, _ := GuardedRemoteOptions(nil, "kyverno", nil, nil, false)
	require.NotEmpty(t, nativeOpts)
}

// A policy that declares Secrets with no lister to read them from must fail loudly rather
// than quietly fetching unauthenticated, which is what registryclient.New would do on its
// own. Reachable from the CLI, which has no cluster client to build a lister from.
func TestRequiredSecretsKeychain(t *testing.T) {
	t.Parallel()

	keychain := requiredSecretsKeychain(nil, "kyverno", []string{"regcred"})
	require.NotNil(t, keychain, "declared Secrets with no lister must still produce a keychain")
	_, err := keychain.Resolve(registryResource("registry.example.com"))
	require.ErrorContains(t, err, "secret lister is not configured")

	// Nothing to enforce: no declared Secrets, so the default chain is correct.
	require.Nil(t, requiredSecretsKeychain(nil, "kyverno", nil))
	// A real lister is handled by registryclient.New itself.
	require.Nil(t, requiredSecretsKeychain(fakeSecretLister{}, "kyverno", []string{"regcred"}))
}

type fakeSecretLister struct{ corev1listers.SecretLister }

func TestValidateConfigMatchesNew(t *testing.T) {
	t.Parallel()
	for _, config := range []Config{
		{},
		{Mode: ModeAudit},
		{Mode: ModeEnforce, Allowlist: []string{"registry.internal", "10.0.0.0/8"}},
		{Mode: "nonsense"},
		{Allowlist: []string{""}},
	} {
		_, newErr := New(config)
		require.Equal(t, newErr != nil, ValidateConfig(config) != nil, "config %+v", config)
	}
}

func TestParseAllowlist(t *testing.T) {
	t.Parallel()
	require.Nil(t, ParseAllowlist(""))
	require.Nil(t, ParseAllowlist("   "))
	require.Equal(t, []string{"a", "b"}, ParseAllowlist("a,b"))
	// A single empty element is preserved so New rejects it rather than silently ignoring it.
	require.Equal(t, []string{"a", "", "b"}, ParseAllowlist("a,,b"))
}
