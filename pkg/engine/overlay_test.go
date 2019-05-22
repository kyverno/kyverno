package engine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"gotest.tools/assert"
)

func TestApplyOverlay_BaseCase(t *testing.T) {
	resource1Raw := []byte(`{ "dictionary": { "key1": "val1", "key2": "val2", "array": [ 1, 2 ] } }`)
	resource2Raw := []byte(`{ "dictionary": "somestring" }`)

	var resource1, resource2 interface{}

	json.Unmarshal(resource1Raw, &resource1)
	json.Unmarshal(resource2Raw, &resource2)

	fmt.Printf("First resource type: %v", reflect.TypeOf(resource1))
	fmt.Printf("Second resource type: %v", reflect.TypeOf(resource2))

	assert.Assert(t, reflect.TypeOf(resource1) == reflect.TypeOf(resource2))
}
