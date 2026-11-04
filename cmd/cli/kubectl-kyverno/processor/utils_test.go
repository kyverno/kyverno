package processor

import (
	"testing"

	"github.com/kyverno/kyverno/ext/cluster"
	"github.com/kyverno/kyverno/pkg/cel/libs"
	"gotest.tools/v3/assert"
)

func TestNewContextProvider_ClusterModeServesGlobalContextEntries(t *testing.T) {
	prev := libs.LibraryContext
	t.Cleanup(func() { libs.LibraryContext = prev })
	dclient, err := cluster.NewFake().DClient(nil)
	assert.NilError(t, err)
	entries := map[string]interface{}{
		"widgets": map[string]interface{}{"names": []interface{}{"a", "b"}},
		"scalar":  "value",
	}

	provider, err := NewContextProvider(dclient, nil, nil, "", false, false, entries, nil)
	assert.NilError(t, err)

	whole, err := provider.GetGlobalReference("widgets", "")
	assert.NilError(t, err)
	assert.DeepEqual(t, whole, entries["widgets"])

	names, err := provider.GetGlobalReference("widgets", "names")
	assert.NilError(t, err)
	assert.DeepEqual(t, names, []interface{}{"a", "b"})

	_, err = provider.GetGlobalReference("widgets", "missing")
	assert.ErrorContains(t, err, `projection "missing" not found in global context entry "widgets"`)

	_, err = provider.GetGlobalReference("scalar", "names")
	assert.ErrorContains(t, err, "stored value is not an object")

	unknown, err := provider.GetGlobalReference("unknown", "")
	assert.NilError(t, err)
	assert.Assert(t, unknown == nil)
}
