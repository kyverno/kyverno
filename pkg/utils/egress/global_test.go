package egress

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/stretchr/testify/require"
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

// proxiedGuard builds a Guard whose transport routes through a proxy, without touching the
// process environment: http.ProxyFromEnvironment caches its answer for the life of the
// process, so t.Setenv could not steer it.
func proxiedGuard(t *testing.T, config Config, answers map[string][]string, proxy bool) *Guard {
	t.Helper()
	policy := testPolicy(t, config, answers)
	base := &http.Transport{}
	if proxy {
		base.Proxy = http.ProxyURL(&url.URL{Scheme: "http", Host: "proxy.example"})
	}
	return &Guard{policy: policy, transport: policy.WrapTransport(base)}
}

// A keychain is resolved before the transport dials, so the transport's "leave it to the
// proxy" carve-out is only reachable if the keychain makes the same allowance. Without this
// the registry the docs promise is reachable fails during credential resolution instead.
func TestGuardKeychainDefersUnresolvableRegistryToProxy(t *testing.T) {
	t.Parallel()

	resolved := false
	guarded := proxiedGuard(t, Config{Mode: ModeEnforce}, map[string][]string{"proxy.example": {publicIPv4}}, true).
		GuardKeychain(keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
			resolved = true
			return authn.Anonymous, nil
		}))

	// "registry.internal" is absent from the resolver answers, so the local lookup reports
	// not-found -- the one error a configured proxy may excuse.
	_, err := guarded.Resolve(registryResource("registry.internal"))
	require.NoError(t, err)
	require.True(t, resolved, "credentials were not resolved for a proxy-only registry")
}

// The allowance is the proxy's, not the name's: with nothing to defer to, an unresolvable
// registry still fails and no credentials are resolved for it.
func TestGuardKeychainFailsUnresolvableRegistryWithoutProxy(t *testing.T) {
	t.Parallel()

	resolved := false
	guarded := proxiedGuard(t, Config{Mode: ModeEnforce}, nil, false).
		GuardKeychain(keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
			resolved = true
			return authn.Anonymous, nil
		}))

	_, err := guarded.Resolve(registryResource("registry.internal"))
	require.Error(t, err)
	require.False(t, resolved, "credentials were resolved for an unvalidated registry")
}

// Deferring must not become a way around the hard blocks. A literal address never reaches the
// resolver, so it is never a not-found and a proxy cannot excuse it.
func TestGuardKeychainBlocksLiteralAddressBehindProxy(t *testing.T) {
	t.Parallel()

	resolved := false
	guarded := proxiedGuard(t, Config{}, map[string][]string{"proxy.example": {publicIPv4}}, true).
		GuardKeychain(keychainFunc(func(authn.Resource) (authn.Authenticator, error) {
			resolved = true
			return authn.Anonymous, nil
		}))

	_, err := guarded.Resolve(registryResource("169.254.169.254"))
	require.ErrorIs(t, err, ErrAddressBlocked)
	require.False(t, resolved, "credentials were resolved for a blocked registry")
}

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
