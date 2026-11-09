package cache

import (
	"github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
)

// Client caches image verification verdicts. An entry records that every
// attestor in a group verified an image digest: its signature when attestation
// is nil, otherwise that attestation. Entries are keyed by what was verified,
// not by the policy that asked, so policies trusting the same attestors share
// them.
type Client interface {
	// Get reports whether every attestor verified imageRef, and returns the
	// verified attestation payloads stored with the entry: intoto payloads keyed
	// by predicate type, or verified referrer descriptors keyed by artifact
	// type. Payloads are nil for signature entries.
	Get(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation) (found bool, payloads map[string][]byte, err error)

	// Set records that every attestor verified imageRef, along with the
	// verified payloads. The entry expires after the cache TTL. Returns true
	// when the cache holds the entry afterwards.
	Set(imageRef string, attestors []v1beta1.Attestor, attestation *v1beta1.Attestation, payloads map[string][]byte) (bool, error)
}
