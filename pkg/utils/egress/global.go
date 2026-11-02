package egress

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/sdk/extensions/regcreds"
)

// credentialValidationTimeout bounds the destination check made before credentials are
// resolved. Credential resolution is not part of the registry request, so it carries no
// deadline of its own on every code path.
const credentialValidationTimeout = 5 * time.Second

// Guard is the process-wide egress policy and the transport built from it. One Guard serves
// every registry client in the process, so clients configured by the operator share
// connection pools while keeping their own credentials.
type Guard struct {
	policy    *Policy
	transport *Transport
}

var global atomic.Pointer[Guard]

// ParseAllowlist splits the comma-separated operator flag. An empty flag means no
// exemptions, which is distinct from a single empty entry: the latter is rejected by New.
func ParseAllowlist(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// ValidateConfig reports whether operator settings are usable, so a controller can fail
// startup instead of discovering a bad allowlist on the first registry request.
func ValidateConfig(config Config) error {
	_, err := New(config)
	return err
}

// SetGlobal builds the process-wide Guard. Controllers call it during setup, before any
// registry client is constructed. Calling it more than once replaces the Guard; clients
// already holding the previous transport keep using it.
func SetGlobal(config Config) error {
	policy, err := New(config)
	if err != nil {
		return err
	}
	global.Store(&Guard{policy: policy, transport: policy.WrapTransport(regcreds.DefaultTransport)})
	return nil
}

// Global returns the process-wide Guard, or nil when no controller has installed one. A nil
// Guard is usable: its methods leave the registry client at the SDK's native behavior, which
// is what the CLI and unit tests want.
func Global() *Guard {
	return global.Load()
}

// RestoreGlobal reinstalls a Guard captured from Global, including a nil one. SetGlobal builds
// a Guard from configuration and so cannot express "no Guard at all"; a caller that installs
// one temporarily uses this to put process-wide state back exactly as it found it.
func RestoreGlobal(guard *Guard) {
	global.Store(guard)
}

// Transport returns the guarded round tripper, or nil to leave the SDK's default in place.
// Pass it to registryclient.WithTransport.
func (g *Guard) Transport() http.RoundTripper {
	if g == nil {
		return nil
	}
	return g.transport
}

// Policy returns the destination policy, or nil when no Guard is installed.
func (g *Guard) Policy() *Policy {
	if g == nil {
		return nil
	}
	return g.policy
}

// GuardKeychain validates a registry destination before inner resolves credentials for it,
// so a blocked host never receives them. Pass it to registryclient.WithKeychainWrapper.
func (g *Guard) GuardKeychain(inner authn.Keychain) authn.Keychain {
	if g == nil || g.policy == nil {
		return inner
	}
	return &guardedKeychain{inner: inner, policy: g.policy, transport: g.transport}
}

type guardedKeychain struct {
	inner     authn.Keychain
	policy    *Policy
	transport *Transport
}

var _ authn.ContextKeychain = (*guardedKeychain)(nil)

func (g *guardedKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	return g.ResolveContext(context.Background(), resource)
}

func (g *guardedKeychain) ResolveContext(ctx context.Context, resource authn.Resource) (authn.Authenticator, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	validationCtx, cancel := context.WithTimeout(ctx, credentialValidationTimeout)
	_, err := g.policy.ResolveAndValidate(validationCtx, resource.RegistryStr())
	cancel()
	if err != nil {
		// The transport leaves a registry only the proxy can resolve to the proxy. A keychain
		// is resolved before the transport dials, so returning here would make that carve-out
		// unreachable and the destination unusable after all. This matches what the transport
		// then does rather than relying on it: for this same name it also skips the target
		// check and hands the request to the proxy. So nothing about the final destination is
		// validated on this path, and these credentials travel with that request. The carve-out
		// is bounded by its precondition -- a proxy is configured and the local resolver cannot
		// find the name, which is the operator delegating resolution to the proxy.
		if !dnsNotFound(err) || !g.proxied(resource.RegistryStr()) {
			return nil, err
		}
		logging.WithName("egress").V(2).Info("resolving credentials for a destination left to the proxy's resolver", "registry", resource.RegistryStr())
	}
	return authn.Resolve(ctx, g.inner, resource)
}

// proxied reports whether a proxy is configured for this registry. A keychain is told only a
// host, so both schemes are tried: go-containerregistry picks http for an insecure registry and
// https otherwise, and either match is enough to know the transport will not dial it directly.
// Erring towards true only defers a check the transport still makes.
func (g *guardedKeychain) proxied(registry string) bool {
	if g.transport == nil || g.transport.proxy == nil {
		return false
	}
	for _, scheme := range []string{"https", "http"} {
		proxy, err := g.transport.proxy(&http.Request{URL: &url.URL{Scheme: scheme, Host: registry}})
		if err == nil && proxy != nil {
			return true
		}
	}
	return false
}
