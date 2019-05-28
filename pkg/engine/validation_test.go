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

	assert.Assert(t, checkForWildcard(value, pattern) != nil)
	assert.Assert(t, checkForWildcard(middle, pattern) != nil)
}

func TestCheckForWildcard_MiddleAsteriskTest(t *testing.T) {
	pattern := "ab*ba"
	value := "abbeba"
	assert.NilError(t, checkForWildcard(value, pattern))

	value = "abbca"
	assert.Assert(t, checkForWildcard(value, pattern) != nil)
}

func TestCheckForWildcard_QuestionMark(t *testing.T) {
	pattern := "ab?ba"
	value := "abbba"
	assert.NilError(t, checkForWildcard(value, pattern))

	value = "abbbba"
	assert.Assert(t, checkForWildcard(value, pattern) != nil)
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

	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Equal(t, len(actualMap), 2)
	assert.Equal(t, actualMap["(name)"].(string), "nirmata-*")
	assert.Equal(t, actualMap["(namespace)"].(string), "kube-?olicy")
}

func TestGetAnchorsFromMap_ThereAreNoAnchors(t *testing.T) {
	rawMap := []byte(`{"name": "nirmata-*", "notAnchor1": 123, "namespace": "kube-?olicy", "notAnchor2": "sample-text", "object": { "key1": "value1", "(key2)": "value2"}}`)

	var unmarshalled map[string]interface{}
	json.Unmarshal(rawMap, &unmarshalled)

	actualMap := GetAnchorsFromMap(unmarshalled)
	assert.Assert(t, len(actualMap) == 0)
}
