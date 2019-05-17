package engine

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func TestWrappedWithParentheses_StringIsWrappedWithParentheses(t *testing.T) {
	str := "(something)"
	assert.Assert(t, wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasOnlyParentheses(t *testing.T) {
	str := "()"
	assert.Assert(t, wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasNoParentheses(t *testing.T) {
	str := "something"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasLeftParentheses(t *testing.T) {
	str := "(something"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringHasRightParentheses(t *testing.T) {
	str := "something)"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_StringParenthesesInside(t *testing.T) {
	str := "so)m(et(hin)g"
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestWrappedWithParentheses_Empty(t *testing.T) {
	str := ""
	assert.Assert(t, !wrappedWithParentheses(str))
}

func TestCheckForWildcard_AsteriskTest(t *testing.T) {
	pattern := "*"
	value := "anything"
	empty := ""

	assert.Assert(t, checkForWildcard(value, pattern))
	assert.Assert(t, checkForWildcard(empty, pattern))
}

func TestCheckForWildcard_LeftAsteriskTest(t *testing.T) {
	pattern := "*right"
	value := "leftright"
	right := "right"

	assert.Assert(t, checkForWildcard(value, pattern))
	assert.Assert(t, checkForWildcard(right, pattern))

	value = "leftmiddle"
	middle := "middle"

	assert.Assert(t, !checkForWildcard(value, pattern))
	assert.Assert(t, !checkForWildcard(middle, pattern))
}

func TestCheckForWildcard_MiddleAsteriskTest(t *testing.T) {
	pattern := "ab*ba"
	value := "abbba"
	assert.Assert(t, checkForWildcard(value, pattern))

	value = "abbca"
	assert.Assert(t, !checkForWildcard(value, pattern))
}

func TestCheckForWildcard_QuestionMark(t *testing.T) {
	pattern := "ab?ba"
	value := "abbba"
	assert.Assert(t, checkForWildcard(value, pattern))

	value = "abbbba"
	assert.Assert(t, !checkForWildcard(value, pattern))
}

func TestCheckSingleValue_CheckInt(t *testing.T) {
	pattern := 89
	value := 89
	assert.Assert(t, checkSingleValue(value, pattern))

	value = 202
	assert.Assert(t, !checkSingleValue(value, pattern))
}

func TestCheckSingleValue_CheckFloat(t *testing.T) {
	pattern := 89.9091
	value := 89.9091
	assert.Assert(t, checkSingleValue(value, pattern))

	value = 89.9092
	assert.Assert(t, !checkSingleValue(value, pattern))
}

func TestCheckSingleValue_CheckOperatorMoreEqual(t *testing.T) {
	pattern := "  >=    89 "
	value := 89
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = ">=10.0001"
	floatValue := 89.901
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorMoreEqualFail(t *testing.T) {
	pattern := "  >=    90 "
	value := 89
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = ">=910.0001"
	floatValue := 89.901
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLessEqual(t *testing.T) {
	pattern := "   <=  1 "
	value := 1
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = "<=10.0001"
	floatValue := 1.901
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLessEqualFail(t *testing.T) {
	pattern := "   <=  0.1558 "
	value := 1
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = "<=10.0001"
	floatValue := 12.901
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorMore(t *testing.T) {
	pattern := "   >  10 "
	value := 89
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = ">10.0001"
	floatValue := 89.901
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorMoreFail(t *testing.T) {
	pattern := "   >  89 "
	value := 89
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = ">910.0001"
	floatValue := 89.901
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLess(t *testing.T) {
	pattern := "   <  10 "
	value := 9
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = "<10.0001"
	floatValue := 9.901
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLessFail(t *testing.T) {
	pattern := "   <  10 "
	value := 10
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = "<10.0001"
	floatValue := 19.901
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorNotEqual(t *testing.T) {
	pattern := "   !=  10 "
	value := 9.99999
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = "!=10.0001"
	floatValue := 10.0000
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorNotEqualFail(t *testing.T) {
	pattern := "   !=  9.99999 "
	value := 9.99999
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = "!=10"
	floatValue := 10
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorEqual(t *testing.T) {
	pattern := "     10.000001 "
	value := 10.000001
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = "10.000000"
	floatValue := 10
	assert.Assert(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorEqualFail(t *testing.T) {
	pattern := "     10.000000 "
	value := 10.000001
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = "10.000001"
	floatValue := 10
	assert.Assert(t, !checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckSeveralOperators(t *testing.T) {
	pattern := " <-1  |  10.000001 "
	value := 10.000001
	assert.Assert(t, checkSingleValue(value, pattern))

	value = -30
	assert.Assert(t, checkSingleValue(value, pattern))

	value = 5
	assert.Assert(t, !checkSingleValue(value, pattern))
}

func TestCheckSingleValue_CheckWildcard(t *testing.T) {
	pattern := "nirmata_*"
	value := "nirmata_awesome"
	assert.Assert(t, checkSingleValue(value, pattern))

	pattern = "nirmata_*"
	value = "spasex_awesome"
	assert.Assert(t, !checkSingleValue(value, pattern))

	pattern = "g?t"
	value = "git"
	assert.Assert(t, checkSingleValue(value, pattern))
}

func TestSkipArrayObject_OneAnchor(t *testing.T) {

	rawAnchors := []byte(`{"(name)": "nirmata-*"}`)
	rawResource := []byte(`{"name": "nirmata-resource", "namespace": "kube-policy", "object": { "label": "app", "array": [ 1, 2, 3 ]}}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_OneNumberAnchorPass(t *testing.T) {

	rawAnchors := []byte(`{"(count)": 1}`)
	rawResource := []byte(`{"name": "nirmata-resource", "count": 1, "namespace": "kube-policy", "object": { "label": "app", "array": [ 1, 2, 3 ]}}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_TwoAnchorsPass(t *testing.T) {
	rawAnchors := []byte(`{"(name)": "nirmata-*", "(namespace)": "kube-?olicy"}`)
	rawResource := []byte(`{"name": "nirmata-resource", "namespace": "kube-policy", "object": { "label": "app", "array": [ 1, 2, 3 ]}}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, !skipArrayObject(resource, anchor))
}

func TestSkipArrayObject_TwoAnchorsSkip(t *testing.T) {
	rawAnchors := []byte(`{"(name)": "nirmata-*", "(namespace)": "some-?olicy"}`)
	rawResource := []byte(`{"name": "nirmata-resource", "namespace": "kube-policy", "object": { "label": "app", "array": [ 1, 2, 3 ]}}`)

	var resource, anchor map[string]interface{}

	json.Unmarshal(rawAnchors, &anchor)
	json.Unmarshal(rawResource, &resource)

	assert.Assert(t, skipArrayObject(resource, anchor))
}

func TestGetAnchorsFromMap_ThereAreAnchors(t *testing.T) {
	rawMap := []byte(`{"(name)": "nirmata-*", "notAnchor1": 123, "(namespace)": "kube-?olicy", "notAnchor2": "sample-text", "object": { "key1": "value1", "(key2)": "value2"}}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap, err := getAnchorsFromMap(unmarshalled)
	assert.NilError(t, err)
	assert.Equal(t, len(actualMap), 2)
	assert.Equal(t, actualMap["(name)"].(string), "nirmata-*")
	assert.Equal(t, actualMap["(namespace)"].(string), "kube-?olicy")
}

func TestGetAnchorsFromMap_ThereAreNoAnchors(t *testing.T) {
	rawMap := []byte(`{"name": "nirmata-*", "notAnchor1": 123, "namespace": "kube-?olicy", "notAnchor2": "sample-text", "object": { "key1": "value1", "(key2)": "value2"}}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap, err := getAnchorsFromMap(unmarshalled)
	assert.NilError(t, err)
	assert.Assert(t, len(actualMap) == 0)
}

func TestValidateMapElement_TwoElementsInArrayOnePass(t *testing.T) {
	rawPattern := []byte(`[ { "(name)": "nirmata-*", "object": [ { "(key1)": "value*", "key2": "value*" } ] } ]`)
	rawMap := []byte(`[ { "name": "nirmata-1", "object": [ { "key1": "value1", "key2": "value2" } ] }, { "name": "nirmata-1", "object": [ { "key1": "not_value", "key2": "not_value" } ] } ]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	assert.Assert(t, validateMapElement(resource, pattern))
}

func TestValidateMapElement_OneElementInArrayPass(t *testing.T) {
	rawPattern := []byte(`[ { "(name)": "nirmata-*", "object": [ { "(key1)": "value*", "key2": "value*" } ] } ]`)
	rawMap := []byte(`[ { "name": "nirmata-1", "object": [ { "key1": "value1", "key2": "value2" } ] } ]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	assert.Assert(t, validateMapElement(resource, pattern))
}

func TestValidateMapElement_OneElementInArrayNotPass(t *testing.T) {
	rawPattern := []byte(`[{"(name)": "nirmata-*", "object":[{"(key1)": "value*", "key2": "value*"}]}]`)
	rawMap := []byte(`[ { "name": "nirmata-1", "object": [ { "key1": "value5", "key2": "1value1" } ] } ]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	assert.Assert(t, !validateMapElement(resource, pattern))
}
