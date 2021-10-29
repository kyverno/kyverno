package jsonutils

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
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
