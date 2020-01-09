package utils

import(
	"testing"
	"encoding/json"
	"gotest.tools/assert"
)

func TestGetAnchorsFromMap_ThereAreNoAnchors(t *testing.T) {
	rawMap := []byte(`{
		"name":"nirmata-*",
		"notAnchor1":123,
		"namespace":"kube-?olicy",
		"notAnchor2":"sample-text",
		"object":{
			"key1":"value1",
			"(key2)":"value2"
		}
	}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Assert(t, len(actualMap) == 0)
}