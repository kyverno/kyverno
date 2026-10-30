package egress

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
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
	return &guardedKeychain{inner: inner, policy: g.policy}
}

type guardedKeychain struct {
	inner  authn.Keychain
	policy *Policy
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
		return nil, err
	}
	return authn.Resolve(ctx, g.inner, resource)
}
