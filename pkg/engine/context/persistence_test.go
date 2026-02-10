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
