package wildcards

import (
	"reflect"
	"testing"
)

func TestExpandInMetadata(t *testing.T) {
	//testExpand(t, map[string]string{"test/*": "*"}, map[string]string{},
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
