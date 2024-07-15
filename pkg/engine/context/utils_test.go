package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeMaps(t *testing.T) {
	map1 := map[string]interface{}{
		"strVal":   "bar1",
		"strVal2":  "bar2",
		"intVal":   2,
		"arrayVal": []string{"1", "2", "3"},
		"mapVal": map[string]interface{}{
			"foo": "bar",
		},
		"mapVal2": map[string]interface{}{
			"foo2": map[string]interface{}{
				"foo3": 3,
			},
		},
	}

	map2 := map[string]interface{}{
		"strVal":   "bar2",
		"intVal":   3,
		"intVal2":  3,
		"arrayVal": []string{"1", "2", "3", "4"},
		"mapVal": map[string]interface{}{
			"foo1": "bar1",
			"foo2": "bar2",
		},
	}

	mergeMaps(map1, map2, false)

	assert.Equal(t, "bar1", map2["strVal"])
	assert.Equal(t, "bar2", map2["strVal2"])
	assert.Equal(t, 2, map2["intVal"])
	assert.Equal(t, 3, map2["intVal2"])
	assert.Equal(t, []string{"1", "2", "3"}, map2["arrayVal"])
	assert.Equal(t, map[string]interface{}{"foo": "bar", "foo1": "bar1", "foo2": "bar2"}, map2["mapVal"])
	assert.Equal(t, map1["mapVal2"], map2["mapVal2"])

	requestObj := map[string]interface{}{
		"request": map[string]interface{}{
			"object": map[string]interface{}{
				"foo": "bar",
			},
		},
	}

	ctxMap := map[string]interface{}{}
	mergeMaps(requestObj, ctxMap, false)

	r := ctxMap["request"].(map[string]interface{})
	o := r["object"].(map[string]interface{})
	assert.Equal(t, o["foo"], "bar")

	requestObj2 := map[string]interface{}{
		"request": map[string]interface{}{
			"object": map[string]interface{}{
				"foo":  "bar2",
				"foo2": "bar2",
			},
		},
	}

	mergeMaps(requestObj2, ctxMap, false)
	r2 := ctxMap["request"].(map[string]interface{})
	o2 := r2["object"].(map[string]interface{})
	assert.Equal(t, "bar2", o2["foo"])
	assert.Equal(t, "bar2", o2["foo2"])

	request3 := map[string]interface{}{
		"request": map[string]interface{}{
			"userInfo": "user1",
		},
	}

	mergeMaps(request3, ctxMap, false)
	r3 := ctxMap["request"].(map[string]interface{})
	o3 := r3["object"].(map[string]interface{})
	assert.NotNil(t, o3)
	assert.Equal(t, "bar2", o2["foo"])
	assert.Equal(t, "bar2", o2["foo2"])
	assert.Equal(t, "user1", r3["userInfo"])

	request4 := map[string]interface{}{
		"request": map[string]interface{}{
			"object": map[string]interface{}{
				"foo": "bar3",
			},
		},
	}

	mergeMaps(request4, ctxMap, false)
	r4 := ctxMap["request"].(map[string]interface{})
	assert.NotNil(t, r4)
	assert.Equal(t, "user1", r4["userInfo"])

	request5 := map[string]interface{}{
		"request": map[string]interface{}{
			"object": map[string]interface{}{
				"foo": "bar4",
			},
		},
	}

	mergeMaps(request5, ctxMap, true)
	r5 := ctxMap["request"].(map[string]interface{})
	userInfo := r5["userInfo"]
	assert.Nil(t, userInfo)
}

func TestStructToUntypedMap(t *testing.T) {
	type SampleStuct struct {
		Name string `json:"name"`
		ID   int32  `json:"identifier"`
	}

	sample := &SampleStuct{
		Name: "user1",
		ID:   12345,
	}

	result, err := toUnstructured(sample)
	assert.Nil(t, err)

	assert.Equal(t, "user1", result["name"])
	assert.Equal(t, int64(12345), result["identifier"])
}

func TestClearLeaf(t *testing.T) {
	request := map[string]interface{}{
		"request": map[string]interface{}{
			"object": map[string]interface{}{
				"key1": "val1",
				"key2": "val2",
			},
		},
	}

	result := clearLeafValue(request, "request", "object", "key1")
	assert.True(t, result)

	r := request["request"].(map[string]interface{})
	o := r["object"].(map[string]interface{})
	_, exists := o["key1"]
	assert.Equal(t, false, exists)

	_, exists = o["key2"]
	assert.Equal(t, true, exists)

	result = clearLeafValue(request, "request", "object", "key3")
	assert.True(t, result)

	_, exists = o["key3"]
	assert.Equal(t, false, exists)

	result = clearLeafValue(request, "request", "object-bad", "key3")
	assert.Equal(t, false, result)

	result = clearLeafValue(request, "request", "object")
	assert.True(t, result)

	_, exists = r["object"]
	assert.Equal(t, false, exists)
}
