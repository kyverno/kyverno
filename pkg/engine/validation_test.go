package engine

import (
	"encoding/json"
	"testing"

	kubepolicy "github.com/nirmata/kyverno/pkg/apis/policy/v1alpha1"
	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestCheckSingleValue_CheckInt(t *testing.T) {
	pattern := 89
	value := 89
	assert.NilError(t, checkSingleValue(value, pattern))

	value = 202
	assert.Assert(t, checkSingleValue(value, pattern) != nil)
}

func TestCheckSingleValue_CheckFloat(t *testing.T) {
	pattern := 89.9091
	value := 89.9091
	assert.NilError(t, checkSingleValue(value, pattern))

	value = 89.9092
	assert.Assert(t, checkSingleValue(value, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorMoreEqual(t *testing.T) {
	pattern := "  >=    89 "
	value := 89
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = ">=10.0001"
	floatValue := 89.901
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorMoreEqualFail(t *testing.T) {
	pattern := "  >=    90 "
	value := 89
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = ">=910.0001"
	floatValue := 89.901
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorLessEqual(t *testing.T) {
	pattern := "   <=  1 "
	value := 1
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = "<=10.0001"
	floatValue := 1.901
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLessEqualFail(t *testing.T) {
	pattern := "   <=  0.1558 "
	value := 1
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = "<=10.0001"
	floatValue := 12.901
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorMore(t *testing.T) {
	pattern := "   >  10 "
	value := 89
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = ">10.0001"
	floatValue := 89.901
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorMoreFail(t *testing.T) {
	pattern := "   >  89 "
	value := 89
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = ">910.0001"
	floatValue := 89.901
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorLess(t *testing.T) {
	pattern := "   <  10 "
	value := 9
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = "<10.0001"
	floatValue := 9.901
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorLessFail(t *testing.T) {
	pattern := "   <  10 "
	value := 10
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = "<10.0001"
	floatValue := 19.901
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorNotEqual(t *testing.T) {
	pattern := "   !=  10 "
	value := 9.99999
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = "!=10.0001"
	floatValue := 10.0000
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorNotEqualFail(t *testing.T) {
	pattern := "   !=  9.99999 "
	value := 9.99999
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = "!=10"
	floatValue := 10
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckOperatorEqual(t *testing.T) {
	pattern := "     10.000001 "
	value := 10.000001
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = "10.000000"
	floatValue := 10
	assert.NilError(t, checkSingleValue(floatValue, pattern))
}

func TestCheckSingleValue_CheckOperatorEqualFail(t *testing.T) {
	pattern := "     10.000000 "
	value := 10.000001
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = "10.000001"
	floatValue := 10
	assert.Assert(t, checkSingleValue(floatValue, pattern) != nil)
}

func TestCheckSingleValue_CheckSeveralOperators(t *testing.T) {
	pattern := " <-1  |  10.000001 "
	value := 10.000001
	assert.NilError(t, checkSingleValue(value, pattern))

	value = -30
	assert.NilError(t, checkSingleValue(value, pattern))

	value = 5
	assert.Assert(t, checkSingleValue(value, pattern) != nil)
}

func TestCheckSingleValue_CheckWildcard(t *testing.T) {
	pattern := "nirmata_*"
	value := "nirmata_awesome"
	assert.NilError(t, checkSingleValue(value, pattern))

	pattern = "nirmata_*"
	value = "spasex_awesome"
	assert.Assert(t, checkSingleValue(value, pattern) != nil)

	pattern = "g?t"
	value = "git"
	assert.NilError(t, checkSingleValue(value, pattern))
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

	assert.NilError(t, validateMapElement(resource, pattern))
}

func TestValidateMapElement_OneElementInArrayPass(t *testing.T) {
	rawPattern := []byte(`[ { "(name)": "nirmata-*", "object": [ { "(key1)": "value*", "key2": "value*" } ] } ]`)
	rawMap := []byte(`[ { "name": "nirmata-1", "object": [ { "key1": "value1", "key2": "value2" } ] } ]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	assert.NilError(t, validateMapElement(resource, pattern))
}

func TestValidateMapElement_OneElementInArrayNotPass(t *testing.T) {
	rawPattern := []byte(`[{"(name)": "nirmata-*", "object":[{"(key1)": "value*", "key2": "value*"}]}]`)
	rawMap := []byte(`[ { "name": "nirmata-1", "object": [ { "key1": "value5", "key2": "1value1" } ] } ]`)

	var pattern, resource interface{}
	json.Unmarshal(rawPattern, &pattern)
	json.Unmarshal(rawMap, &resource)

	assert.Assert(t, validateMapElement(resource, pattern) != nil)
}

func TestValidate_ServiceTest(t *testing.T) {
	rawPolicy := []byte(`{ "apiVersion": "kyverno.nirmata.io/v1alpha1", "kind": "Policy", "metadata": { "name": "policy-service" }, "spec": { "rules": [ { "name": "ps1", "resource": { "kind": "Service", "name": "game-service*" }, "mutate": { "patches": [ { "path": "/metadata/labels/isMutated", "op": "add", "value": "true" }, { "path": "/metadata/labels/secretLabel", "op": "replace", "value": "weKnow" }, { "path": "/metadata/labels/originalLabel", "op": "remove" }, { "path": "/spec/selector/app", "op": "replace", "value": "mutedApp" } ] }, "validate": { "message": "This resource is broken", "pattern": { "spec": { "ports": [ { "name": "hs", "protocol": 32 } ] } } } } ] } }`)
	rawResource := []byte(`{ "kind": "Service", "apiVersion": "v1", "metadata": { "name": "game-service", "labels": { "originalLabel": "isHere", "secretLabel": "thisIsMySecret" } }, "spec": { "selector": { "app": "MyApp" }, "ports": [ { "name": "http", "protocol": "TCP", "port": 80, "targetPort": 9376 } ] } }`)

	var policy kubepolicy.Policy
	json.Unmarshal(rawPolicy, &policy)

	gvk := metav1.GroupVersionKind{
		Kind: "Service",
	}

	assert.Assert(t, Validate(policy, rawResource, gvk) != nil)
}

func TestValidate_MapHasFloats(t *testing.T) {
	rawPolicy := []byte(`{ "apiVersion": "kyverno.nirmata.io/v1alpha1", "kind": "Policy", "metadata": { "name": "policy-deployment-changed" }, "spec": { "rules": [ { "name": "First policy v2", "resource": { "kind": "Deployment", "name": "nginx-*" }, "mutate": { "patches": [ { "path": "/metadata/labels/isMutated", "op": "add", "value": "true" }, { "path": "/metadata/labels/app", "op": "replace", "value": "nginx_is_mutated" } ] }, "validate": { "message": "replicas number is wrong", "pattern": { "metadata": { "labels": { "app": "*" } }, "spec": { "replicas": 3 } } } } ] } }`)
	rawResource := []byte(`{ "apiVersion": "apps/v1", "kind": "Deployment", "metadata": { "name": "nginx-deployment", "labels": { "app": "nginx" } }, "spec": { "replicas": 3, "selector": { "matchLabels": { "app": "nginx" } }, "template": { "metadata": { "labels": { "app": "nginx" } }, "spec": { "containers": [ { "name": "nginx", "image": "nginx:1.7.9", "ports": [ { "containerPort": 80 } ] } ] } } } }`)

	var policy kubepolicy.Policy
	json.Unmarshal(rawPolicy, &policy)

	gvk := metav1.GroupVersionKind{
		Kind: "Deployment",
	}

	assert.NilError(t, Validate(policy, rawResource, gvk))
}
