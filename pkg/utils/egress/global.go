package egress

import (
	"net/http"
	"sync/atomic"

	"github.com/kyverno/sdk/extensions/regcreds"
)

// Guard is the process-wide egress policy and the transport built from it. One Guard serves
// every registry client, so they share connection pools while keeping their own credentials.
type Guard struct {
	policy    *Policy
	transport *Transport
}

var global atomic.Pointer[Guard]

// ValidateMode checks the mode flag at startup. The denylists are not checked here: they come
// from a ConfigMap read later, where New logs and skips a bad entry instead of failing.
func ValidateMode(mode string) error {
	_, err := New(Config{Mode: mode})
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

// RestoreGlobal reinstalls a Guard captured from Global, including a nil one, which SetGlobal
// cannot express. Used to put process-wide state back as it was found.
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
//
// Credential resolution is deliberately not guarded: every request a credential could ride on
// already goes through Transport. A second check at the keychain would only decide whether
// credentials are selected for a host they can never be sent to, and the two copies of the
// proxy-deferral carve-out had to be kept in sync by hand.
func (g *Guard) Policy() *Policy {
	if g == nil {
		return nil
	}
	return g.policy
}
