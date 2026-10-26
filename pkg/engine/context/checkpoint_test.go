package context

import (
	stdjson "encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCheckpointIsolatesAddResource(t *testing.T) {
	ctx := newContext()
	newObj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "new-secret"},
		"data":     map[string]interface{}{"foo": "new"},
	}
	oldObj := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "new-secret"},
		"data":     map[string]interface{}{"foo": "old"},
	}
	assert.NilError(t, AddResource(ctx, mustJSON(newObj)))
	assert.NilError(t, AddOldResource(ctx, mustJSON(oldObj)))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddResource(oldObj))
	ctx.Restore()

	gotNew, err := ctx.Query("request.object.data.foo")
	assert.NilError(t, err)
	assert.Equal(t, "new", gotNew)

	gotOld, err := ctx.Query("request.oldObject.data.foo")
	assert.NilError(t, err)
	assert.Equal(t, "old", gotOld)
}

func TestCheckpointIsolatesNestedMerge(t *testing.T) {
	ctx := newContext()
	assert.NilError(t, AddResource(ctx, mustJSON(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":   "pod",
			"labels": map[string]interface{}{"app": "test"},
		},
	})))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.labels.foo", "bar"))
	ctx.Restore()

	labels, err := ctx.Query("request.object.metadata.labels")
	assert.NilError(t, err)
	labelMap, ok := labels.(map[string]interface{})
	assert.Assert(t, ok)
	_, hasFoo := labelMap["foo"]
	assert.Assert(t, !hasFoo)
	assert.Equal(t, "test", labelMap["app"])
}

func TestNestedCheckpointRestore(t *testing.T) {
	ctx := newContext()
	assert.NilError(t, AddResource(ctx, mustJSON(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "initial"},
	})))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.rule", "outer"))
	ctx.Checkpoint()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.rule", "inner"))
	ctx.Restore()

	rule, err := ctx.Query("request.object.metadata.rule")
	assert.NilError(t, err)
	assert.Equal(t, "outer", rule)

	ctx.Restore()
	metadata, err := ctx.Query("request.object.metadata")
	assert.NilError(t, err)
	metaMap, ok := metadata.(map[string]interface{})
	assert.Assert(t, ok)
	_, hasRule := metaMap["rule"]
	assert.Assert(t, !hasRule)

	name, err := ctx.Query("request.object.metadata.name")
	assert.NilError(t, err)
	assert.Equal(t, "initial", name)
}

func TestResetIsolatesNestedMutation(t *testing.T) {
	ctx := newContext()
	assert.NilError(t, AddResource(ctx, mustJSON(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "pod"},
	})))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.iteration", "first"))
	ctx.Reset()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.iteration", "second"))
	ctx.Reset()
	assert.NilError(t, ctx.AddVariable("request.object.metadata.iteration", "third"))
	ctx.Restore()

	metadata, err := ctx.Query("request.object.metadata")
	assert.NilError(t, err)
	metaMap, ok := metadata.(map[string]interface{})
	assert.Assert(t, ok)
	_, hasIter := metaMap["iteration"]
	assert.Assert(t, !hasIter)
}

func TestCheckpointDropsRuleContext(t *testing.T) {
	ctx := newContext()
	assert.NilError(t, AddResource(ctx, mustJSON(map[string]interface{}{
		"metadata": map[string]interface{}{"name": "pod"},
	})))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddContextEntry("myConfigMap", []byte(`{"key":"value"}`)))
	ctx.Restore()

	_, err := ctx.Query("myConfigMap")
	assert.ErrorContains(t, err, `Unknown key "myConfigMap"`)
}

func TestCheckpointIsolatesElementAliasWrite(t *testing.T) {
	ctx := newContext()
	container := map[string]interface{}{
		"name":  "c1",
		"image": "nginx:latest",
	}
	assert.NilError(t, ctx.AddResource(map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []interface{}{container},
		},
	}))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddElement(container, 0, 0))
	assert.NilError(t, ctx.AddVariable("element.name", "mutated"))
	ctx.Restore()

	name, err := ctx.Query("request.object.spec.containers[0].name")
	assert.NilError(t, err)
	assert.Equal(t, "c1", name)
}

func mustJSON(v map[string]interface{}) []byte {
	b, err := stdjson.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func TestCheckpointReloadIsNotDoubleCharged(t *testing.T) {
	ctx := NewContextWithMaxSize(nil, 30)
	entry := []byte(`{"key":"0123456789"}`) // 20 bytes
	assert.NilError(t, ctx.AddContextEntry("entry", entry))

	ctx.Checkpoint()
	assert.NilError(t, ctx.AddContextEntry("other", []byte(`"012345"`))) // 8 bytes -> 28
	ctx.Restore()

	// without restoring the size accounting, this would be 28 + 8 = 36 > 30
	ctx.Checkpoint()
	assert.NilError(t, ctx.AddContextEntry("other", []byte(`"012345"`)))
	ctx.Restore()
}

func TestContextEntryCharges(t *testing.T) {
	ctx := NewContextWithMaxSize(nil, 30)
	entry := []byte(`{"key":"0123456789"}`) // 20 bytes
	assert.NilError(t, ctx.AddContextEntry("added", entry))
	assert.NilError(t, ctx.ReplaceContextEntry("replaced", []byte(`"0123"`))) // 6 bytes -> 26

	ctx.Checkpoint()
	// added data is merged into the existing entry, so it accumulates (26 + 20 = 46)
	assert.ErrorContains(t, ctx.AddContextEntry("added", entry), "context size limit exceeded")
	// replaced data releases the previous charge (26 - 6 + 8 = 28)
	assert.NilError(t, ctx.ReplaceContextEntry("replaced", []byte(`"012345"`)))
	assert.ErrorContains(t, ctx.AddContextEntry("new", []byte(`"0123"`)), "context size limit exceeded")
	ctx.Restore()

	// restoring the checkpoint restores the per-entry charges (20 + 6 = 26)
	assert.NilError(t, ctx.AddContextEntry("new", []byte(`"0"`)))
}

func TestClearingContextEntryIsFree(t *testing.T) {
	ctx := NewContextWithMaxSize(nil, 1)
	assert.NilError(t, ctx.AddContextEntry("entry", []byte(`1`))) // at the limit

	// a null entry holds no data, so clearing is never charged, even at the limit
	assert.NilError(t, ctx.ReplaceContextEntry("entry", []byte("null")))
	assert.NilError(t, ctx.AddContextEntry("entry", []byte(`2`)))
}

func TestClearedContextEntryCanShrink(t *testing.T) {
	ctx := NewContextWithMaxSize(nil, 30)
	assert.NilError(t, ctx.AddContextEntry("a", []byte(`{"key":"0123456789"}`))) // 20 bytes
	assert.NilError(t, ctx.AddContextEntry("b", []byte(`"012345"`)))             // 8 bytes -> 28

	// clearing an entry by replacing it resets its charge, so a reload that shrinks one
	// entry and grows another is charged for what the context actually holds (5 + 20)
	ctx.Checkpoint()
	assert.NilError(t, ctx.ReplaceContextEntry("a", []byte("null")))
	assert.NilError(t, ctx.ReplaceContextEntry("b", []byte("null")))
	assert.NilError(t, ctx.AddContextEntry("a", []byte(`"012"`)))
	assert.NilError(t, ctx.AddContextEntry("b", []byte(`{"key":"0123456789"}`)))
	ctx.Restore()
}
