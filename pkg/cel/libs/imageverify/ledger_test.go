package imageverify

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerificationLedger_StatusDistinguishesNeverCheckedFromFailed(t *testing.T) {
	t.Parallel()
	l := NewVerificationLedger()

	// never recorded
	verified, attempted := l.Status("ghcr.io/kyverno/unseen:latest")
	assert.False(t, verified)
	assert.False(t, attempted, "an image no expression checked must not look like a failed check")

	l.Record("ghcr.io/kyverno/failed:latest", false)
	verified, attempted = l.Status("ghcr.io/kyverno/failed:latest")
	assert.False(t, verified)
	assert.True(t, attempted, "a failed check must be distinguishable from no check at all")

	l.Record("ghcr.io/kyverno/ok:latest", true)
	verified, attempted = l.Status("ghcr.io/kyverno/ok:latest")
	assert.True(t, verified)
	assert.True(t, attempted)
}

// A policy's expressions all share one ledger, so an image verified by one
// expression must stay verified when a later expression checks it against a
// different attestor and fails. Without this the verdict would depend on the
// order the expressions happen to be written in.
func TestVerificationLedger_RecordIsMonotonic(t *testing.T) {
	t.Parallel()
	const image = "ghcr.io/kyverno/test-verify-image:signed"

	t.Run("success then failure", func(t *testing.T) {
		t.Parallel()
		l := NewVerificationLedger()
		l.Record(image, true)
		l.Record(image, false)
		verified, _ := l.Status(image)
		assert.True(t, verified, "a later failed check must not clear an earlier genuine verification")
	})

	t.Run("failure then success", func(t *testing.T) {
		t.Parallel()
		l := NewVerificationLedger()
		l.Record(image, false)
		l.Record(image, true)
		verified, _ := l.Status(image)
		assert.True(t, verified)
	})
}

// The ledger is bound into the CEL environment at compile time. If a compiled
// policy were ever reused across resources, stale entries would let an unverified
// image through, so Reset must actually clear the verified state.
func TestVerificationLedger_ResetClearsVerifiedState(t *testing.T) {
	t.Parallel()
	const image = "ghcr.io/kyverno/test-verify-image:signed"
	l := NewVerificationLedger()
	l.Record(image, true)

	l.Reset()

	verified, attempted := l.Status(image)
	assert.False(t, verified, "a verification from a previous evaluation must not carry over")
	assert.False(t, attempted)

	// still usable afterwards
	l.Record(image, true)
	verified, _ = l.Status(image)
	assert.True(t, verified)
}

// The ledger is optional at the API boundary, so every method has to tolerate a
// nil receiver rather than panicking inside a CEL function.
func TestVerificationLedger_NilReceiverIsSafe(t *testing.T) {
	t.Parallel()
	var l *VerificationLedger
	assert.NotPanics(t, func() {
		l.Record("ghcr.io/kyverno/test-verify-image:signed", true)
		l.Reset()
		verified, attempted := l.Status("ghcr.io/kyverno/test-verify-image:signed")
		assert.False(t, verified)
		assert.False(t, attempted)
	})
}
