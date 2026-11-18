package context

import (
	"testing"

	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/engine/jmespath"
	"github.com/stretchr/testify/assert"
)

func Test_Persistence(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	// Set initial value
	err := ctx.AddContextEntry("foo", []byte(`"bar"`))
	assert.NoError(t, err)

	ctx.Checkpoint()

	// Modify value and persist it
	err = ctx.AddContextEntry("foo", []byte(`"baz"`))
	assert.NoError(t, err)
	ctx.Persist("foo")

	// Restore
	ctx.Restore()

	// Verify "foo" is "baz" (persisted) and not "bar" (the original checkpoint value)
	data, err := ctx.Query("foo")
	assert.NoError(t, err)
	assert.Equal(t, "baz", data)
}

func Test_MultiplePersistence(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	err := ctx.AddContextEntry("foo", []byte(`"bar"`))
	assert.NoError(t, err)
	err = ctx.AddContextEntry("qux", []byte(`"val1"`))
	assert.NoError(t, err)

	ctx.Checkpoint()

	err = ctx.AddContextEntry("foo", []byte(`"baz"`))
	assert.NoError(t, err)
	err = ctx.AddContextEntry("qux", []byte(`"val2"`))
	assert.NoError(t, err)

	// Persist two different keys
	ctx.Persist("foo")
	ctx.Persist("qux")

	ctx.Restore()

	data, err := ctx.Query("foo")
	assert.NoError(t, err)
	assert.Equal(t, "baz", data)

	data, err = ctx.Query("qux")
	assert.NoError(t, err)
	assert.Equal(t, "val2", data)
}

func Test_Persistence_NonExistent(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)
	ctx.Checkpoint()
	ctx.Persist("nonexistent")
	ctx.Restore()
	data, err := ctx.Query("nonexistent")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func Test_RestoreAndClearPersisted(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	// Outer checkpoint, mirroring the per-policy Checkpoint taken by
	// verifyAndPatchImages/validate around a policy's whole rule loop.
	ctx.Checkpoint()
	err := ctx.AddContextEntry("foo", []byte(`"attested"`))
	assert.NoError(t, err)
	ctx.Persist("foo")

	// A nested checkpoint/restore, mirroring the per-rule checkpoint taken
	// within that loop, must still see the persisted value: this is the
	// "survive restores across rules within one policy" behavior Persist
	// exists for, and it must not regress.
	ctx.Checkpoint()
	ctx.Restore()
	data, err := ctx.Query("foo")
	assert.NoError(t, err)
	assert.Equal(t, "attested", data)

	// Unwinding the outer checkpoint must drop the persisted value instead of
	// carrying it forward, since the same context object is reused for the
	// next policy's evaluation (see policycontext.PolicyContext.WithPolicy).
	ctx.RestoreAndClearPersisted()
	data, err = ctx.Query("foo")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func Test_RestoreAndClearPersisted_NoCrossPolicyLeak(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	// Policy A's evaluation persists "attestation" so it survives restores
	// across its own rule loop, then its outer checkpoint unwinds the way
	// verifyAndPatchImages/validate do at the end of a policy's evaluation.
	ctx.Checkpoint()
	err := ctx.AddContextEntry("attestation", []byte(`"policy-a-value"`))
	assert.NoError(t, err)
	ctx.Persist("attestation")
	ctx.RestoreAndClearPersisted()

	// Policy B's evaluation reuses the same JSONContext instance, as
	// policycontext.PolicyContext.WithPolicy does in the webhook (it only
	// swaps the policy reference). It must not observe policy A's persisted
	// value merely because it references a variable with the same name.
	ctx.Checkpoint()
	data, err := ctx.Query("attestation")
	assert.Error(t, err)
	assert.Nil(t, data)
	ctx.RestoreAndClearPersisted()
}

func Test_RestoreAndClearPersisted_PreservesPreCheckpointValue(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	// "sharedVar" pre-exists before this policy's checkpoint -- e.g. a
	// preloaded context entry or variable set by an earlier mechanism, not
	// something this policy's Persist() call introduced.
	err := ctx.AddContextEntry("sharedVar", []byte(`"pre-existing-value"`))
	assert.NoError(t, err)

	// Outer checkpoint, mirroring the per-policy Checkpoint taken by
	// verifyAndPatchImages/validate around a policy's whole rule loop.
	ctx.Checkpoint()

	// A rule overwrites "sharedVar" (e.g. with fetched attestation data) and
	// persists it so the new value survives restores across rules within
	// this policy's rule loop. It also introduces a brand-new key that never
	// existed before this checkpoint.
	err = ctx.AddContextEntry("sharedVar", []byte(`"attested-value"`))
	assert.NoError(t, err)
	ctx.Persist("sharedVar")

	err = ctx.AddContextEntry("newAttestation", []byte(`"new-value"`))
	assert.NoError(t, err)
	ctx.Persist("newAttestation")

	// Unwinding the outer checkpoint at the end of this policy's evaluation
	// must restore "sharedVar" to its pre-checkpoint value -- it is
	// pre-existing shared context data, not attestation leakage -- while
	// "newAttestation", which never existed before this checkpoint, must be
	// absent (the original no-cross-policy-leak guarantee must not regress).
	ctx.RestoreAndClearPersisted()

	data, err := ctx.Query("sharedVar")
	assert.NoError(t, err)
	assert.Equal(t, "pre-existing-value", data)

	data, err = ctx.Query("newAttestation")
	assert.Error(t, err)
	assert.Nil(t, data)
}

func Test_Persistence_WithReset(t *testing.T) {
	jp := jmespath.New(config.NewDefaultConfiguration(false))
	ctx := NewContext(jp)

	err := ctx.AddContextEntry("foo", []byte(`"val1"`))
	assert.NoError(t, err)
	ctx.Checkpoint()
	ctx.Persist("foo")
	ctx.Restore()

	ctx.Checkpoint()
	err = ctx.AddContextEntry("foo", []byte(`"val2"`))
	assert.NoError(t, err)
	ctx.Reset()

	data, err := ctx.Query("foo")
	assert.NoError(t, err)
	assert.Equal(t, "val1", data)
}
