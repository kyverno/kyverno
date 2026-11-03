package egress

import (
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/kyverno/sdk/extensions/regcreds"
)

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
//
// Note that credential resolution is deliberately not guarded. Every request a resolved
// credential could ride on goes through Transport, which validates the destination before the
// first byte leaves, so a second check at the keychain would only decide whether credentials are
// selected for a host they can never be sent to. Keeping the check in one place also removes an
// invariant that had to be mirrored by hand: the keychain and the transport each implemented the
// proxy-deferral carve-out, and a change to either one alone made a proxy-only-resolvable
// registry unusable or the carve-out dead code.
func (g *Guard) Policy() *Policy {
	if g == nil {
		return nil
	}
	return g.policy
}
