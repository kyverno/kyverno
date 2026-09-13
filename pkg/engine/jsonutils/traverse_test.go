package jsonutils

import (
	"reflect"
	"testing"

	"gotest.tools/v3/assert"
)

var document = []byte(`
{
	"kind": "{{request.object.metadata.name1}}",
	"name": "ns-owner-{{request.object.metadata.name}}",
	"data": {
		"rules": [
			{
				"apiGroups": [
					"{{request.object.metadata.name}}"
				],
				"resources": [
					"namespaces"
				],
				"verbs": [
					"*"
				]
			}
		]
	}
}
`)

func Test_TraverseLeafsCheckIfTheyHit(t *testing.T) {
	hitMap := map[string]int{
		"{{request.object.metadata.name1}}":         0,
		"ns-owner-{{request.object.metadata.name}}": 0,
		"{{request.object.metadata.name}}":          0,
		"namespaces":                                0,
		"*":                                         0,
	}

	var originalJSON interface{}
	err := json.Unmarshal(document, &originalJSON)
	assert.NilError(t, err)

	traversal := NewTraversal(originalJSON, OnlyForLeafsAndKeys(func(data *ActionData) (interface{}, error) {
		if key, ok := data.Element.(string); ok {
			hitMap[key]++
		}
		return data.Element, nil
	}))

	_, err = traversal.TraverseJSON()
	assert.NilError(t, err)

	for _, v := range hitMap {
		assert.Equal(t, v, 1)
	}
}

func Test_PathMustBeCorrectEveryTime(t *testing.T) {
	expectedValue := "ns-owner-{{request.object.metadata.name}}"
	expectedPath := "/name"

	var originalJSON interface{}
	err := json.Unmarshal(document, &originalJSON)
	assert.NilError(t, err)

	traversal := NewTraversal(originalJSON, OnlyForLeafsAndKeys(func(data *ActionData) (interface{}, error) {
		if data.Element.(string) == expectedValue {
			assert.Equal(t, expectedPath, data.Path)
		}
		return data.Element, nil
	}))

	_, err = traversal.TraverseJSON()
	assert.NilError(t, err)
}

// Test_TraverseJSONCopiesLeafActionContainerResults pins a safety
// property that pkg/engine/variables and, transitively, the engine
// context's lazy checkpoint materialization rely on: when a leaf action
// (for example variable substitution) returns a map or a slice as the
// replacement for a string leaf, TraverseJSON's own recursion into that
// returned value (traverseJSON's post-action switch, which calls
// datautils.CopyMap for maps and slices.Clone plus recursive
// traverseObject/traverseList for both) makes the output independent of
// whatever the action returned by reference, all the way down.
//
// This test does not touch variable substitution at all — it drives
// TraverseJSON directly with an action that hands back a shared,
// mutable container (exactly what a JMESPath variable resolver can do)
// and asserts that mutating the shared container after traversal does
// not affect the traversal's output, and vice versa. If a future change
// removes or weakens the CopyMap/slices.Clone calls in traverseJSON,
// this test must fail — it is the standing guard for that property.
func Test_TraverseJSONCopiesLeafActionContainerResults(t *testing.T) {
	shared := map[string]interface{}{
		"labels": map[string]interface{}{
			"app": "x",
			"num": float64(2),
		},
	}

	pattern := map[string]interface{}{
		"metadata": "REPLACE_ME",
	}

	traversal := NewTraversal(pattern, OnlyForLeafsAndKeys(func(data *ActionData) (interface{}, error) {
		if s, ok := data.Element.(string); ok && s == "REPLACE_ME" {
			return shared, nil
		}
		return data.Element, nil
	}))

	result, err := traversal.TraverseJSON()
	assert.NilError(t, err)

	resultMap, ok := result.(map[string]interface{})
	assert.Assert(t, ok)

	resultMetadata, ok := resultMap["metadata"].(map[string]interface{})
	assert.Assert(t, ok)

	// The top-level map must not be the same object as what the action
	// returned.
	assert.Assert(t, reflect.ValueOf(resultMetadata).Pointer() != reflect.ValueOf(shared).Pointer(), "traversal output must not share the top-level container with the action's return value")

	// Mutating the shared container after traversal must not affect the
	// traversal's output, and mutating the output must not affect the
	// shared container — including the nested "labels" map, which pins
	// that the copy is recursive, not shallow.
	shared["injected"] = "boom"
	_, hasInjected := resultMap["metadata"].(map[string]interface{})["injected"]
	assert.Assert(t, !hasInjected, "mutating the action's shared value after traversal must not leak into the traversal output")

	sharedLabels := shared["labels"].(map[string]interface{})
	sharedLabels["num"] = float64(99)
	resultLabels := resultMetadata["labels"].(map[string]interface{})
	assert.Equal(t, resultLabels["num"], float64(2), "nested containers must be independently copied, not shared by reference")
}
