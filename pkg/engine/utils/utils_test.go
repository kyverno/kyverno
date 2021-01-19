package utils

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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
	err := json.Unmarshal(rawMap, &unmarshalled)
	if err != nil {
		t.Error(err)
	}
	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Equal(t, len(actualMap), 0)
}
