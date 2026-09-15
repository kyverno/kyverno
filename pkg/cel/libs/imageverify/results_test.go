package imageverify

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageVerificationResults_StatusDistinguishesNeverCheckedFromFailed(t *testing.T) {
	t.Parallel()
	l := NewImageVerificationResults()

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

// A policy's expressions all share one set of results, so an image verified by one
// expression must stay verified when a later expression checks it against a
// different attestor and fails. Without this the verdict would depend on the
// order the expressions happen to be written in.
func TestImageVerificationResults_RecordIsMonotonic(t *testing.T) {
	t.Parallel()
	const image = "ghcr.io/kyverno/test-verify-image:signed"

	t.Run("success then failure", func(t *testing.T) {
		t.Parallel()
		l := NewImageVerificationResults()
		l.Record(image, true)
		l.Record(image, false)
		verified, _ := l.Status(image)
		assert.True(t, verified, "a later failed check must not clear an earlier genuine verification")
	})

	t.Run("failure then success", func(t *testing.T) {
		t.Parallel()
		l := NewImageVerificationResults()
		l.Record(image, false)
		l.Record(image, true)
		verified, _ := l.Status(image)
		assert.True(t, verified)
	})
}

// The results are shared by every policy in an admission request, so they must be
// safe to write from more than one goroutine even though policies are evaluated
// sequentially today.
func TestImageVerificationResults_ConcurrentRecordAndStatus(t *testing.T) {
	t.Parallel()
	const image = "ghcr.io/kyverno/test-verify-image:signed"
	l := NewImageVerificationResults()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			l.Record(image, i%2 == 0)
		}()
		go func() {
			defer wg.Done()
			l.Status(image)
		}()
	}
	wg.Wait()

	verified, attempted := l.Status(image)
	assert.True(t, verified)
	assert.True(t, attempted)
}

// The results are optional at the API boundary, so every method has to tolerate a
// nil receiver rather than panicking inside a CEL function.
func TestImageVerificationResults_NilReceiverIsSafe(t *testing.T) {
	t.Parallel()
	var l *ImageVerificationResults
	assert.NotPanics(t, func() {
		l.Record("ghcr.io/kyverno/test-verify-image:signed", true)
		verified, attempted := l.Status("ghcr.io/kyverno/test-verify-image:signed")
		assert.False(t, verified)
		assert.False(t, attempted)
	})
}
