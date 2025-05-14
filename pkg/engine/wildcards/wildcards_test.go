package wildcards

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandInMetadata(t *testing.T) {
	// testExpand(t, map[string]string{"test/*": "*"}, map[string]string{},
	//	map[string]string{"test/0": "0"})

	testExpand(t, map[string]string{"test/*": "*"}, map[string]string{"test/test": "test"},
		map[string]interface{}{"test/test": "*"})

	testExpand(t, map[string]string{"=(test/*)": "test"}, map[string]string{"test/test": "test"},
		map[string]interface{}{"=(test/test)": "test"})

	testExpand(t, map[string]string{"test/*": "*"}, map[string]string{"test/test1": "test1"},
		map[string]interface{}{"test/test1": "*"})
}

func testExpand(t *testing.T, patternMap, resourceMap map[string]string, expectedMap map[string]interface{}) {
	result := replaceWildcardsInMapKeys(patternMap, resourceMap)
	if !reflect.DeepEqual(expectedMap, result) {
		t.Errorf("expected %v but received %v", expectedMap, result)
	}
}

func TestGetValueAsStringMap_NilHandling(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		data           interface{}
		expectedKey    string
		expectedResult map[string]string
	}{
		{
			name:           "nil data",
			key:            "test",
			data:           nil,
			expectedKey:    "",
			expectedResult: nil,
		},
		{
			name:           "data is not a map",
			key:            "test",
			data:           "not a map",
			expectedKey:    "",
			expectedResult: nil,
		},
		{
			name:           "key not found",
			key:            "nonexistent",
			data:           map[string]interface{}{"otherKey": "value"},
			expectedKey:    "",
			expectedResult: nil,
		},
		{
			name:           "value is nil",
			key:            "test",
			data:           map[string]interface{}{"test": nil},
			expectedKey:    "",
			expectedResult: nil,
		},
		{
			name:           "value is not a map",
			key:            "test",
			data:           map[string]interface{}{"test": "not a map"},
			expectedKey:    "",
			expectedResult: nil,
		},
		{
			name: "handles nil value in map",
			key:  "test",
			data: map[string]interface{}{
				"test": map[string]interface{}{
					"key1": "value1",
					"key2": nil,
					"key3": "value3",
				},
			},
			expectedKey: "test",
			expectedResult: map[string]string{
				"key1": "value1",
				// key2 should be skipped
				"key3": "value3",
			},
		},
		{
			name: "handles non-string value in map",
			key:  "test",
			data: map[string]interface{}{
				"test": map[string]interface{}{
					"key1": "value1",
					"key2": 123,
					"key3": map[string]string{
						"nested": "value",
					},
					"key4": "value4",
				},
			},
			expectedKey: "test",
			expectedResult: map[string]string{
				"key1": "value1",
				// key2 should be skipped (non-string)
				// key3 should be skipped (complex)
				"key4": "value4",
			},
		},
		{
			name: "normal case - all strings",
			key:  "test",
			data: map[string]interface{}{
				"test": map[string]interface{}{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expectedKey: "test",
			expectedResult: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			key, result := getValueAsStringMap(test.key, test.data)
			assert.Equal(t, test.expectedKey, key)
			assert.Equal(t, test.expectedResult, result)
		})
	}
}

func TestExpandInMetadata_NilSafety(t *testing.T) {
	testCases := []struct {
		name        string
		patternMap  map[string]interface{}
		resourceMap map[string]interface{}
		shouldPanic bool
	}{
		{
			name: "nil value in annotation should not panic",
			patternMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"some-key": nil,
					},
				},
			},
			resourceMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"real-key": "real-value",
					},
				},
			},
			shouldPanic: false,
		},
		{
			name: "complex value in annotation should not panic",
			patternMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"some-key": map[string]interface{}{
							"nested": "value",
						},
					},
				},
			},
			resourceMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"real-key": "real-value",
					},
				},
			},
			shouldPanic: false,
		},
		{
			name: "simulated jmespath nil result",
			patternMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"test": nil, // Simulating what happens when {{@ | foo}} evaluates with undefined 'foo'
					},
				},
			},
			resourceMap: map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						"real-key": "real-value",
					},
				},
			},
			shouldPanic: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tc.shouldPanic {
					assert.NotNil(t, r, "Expected function to panic, but it didn't")
				} else {
					assert.Nil(t, r, "Function panicked unexpectedly")
				}
			}()

			ExpandInMetadata(tc.patternMap, tc.resourceMap)
		})
	}
}

func TestJMESPathNil(t *testing.T) {
	// Create a pattern with a nil value in labels or annotations
	// to simulate what happens after a JMESPath expression like {{@ | foo}}
	// (where 'foo' is not a defined function) is evaluated and results in nil.
	patternMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"normal":    "value",
				"nil-value": nil, // This represents the result of {{@ | foo}} substitution
			},
			"annotations": map[string]interface{}{
				"another-normal": "value",
				"complex-value": map[string]interface{}{ // And this represents a complex structure
					"nested": "value",
				},
			},
		},
	}

	resourceMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": map[string]interface{}{
				"app": "test",
			},
			"annotations": map[string]interface{}{
				"test": "value",
			},
		},
	}

	defer func() {
		r := recover()
		assert.Nil(t, r, "ExpandInMetadata should not panic with nil values, but it did: %v", r)
	}()

	result := ExpandInMetadata(patternMap, resourceMap)

	// Additional verification that the function works correctly
	metadataResult := result["metadata"].(map[string]interface{})
	labelsResult, ok := metadataResult["labels"].(map[string]interface{})

	assert.True(t, ok, "Expected labels to be a map[string]interface{}")
	assert.Contains(t, labelsResult, "normal")

	// Annotations with complex values should also be handled properly
	annotationsResult, ok := metadataResult["annotations"].(map[string]interface{})
	assert.True(t, ok, "Expected annotations to be a map[string]interface{}")
	assert.Contains(t, annotationsResult, "another-normal")
}
