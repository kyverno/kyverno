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
