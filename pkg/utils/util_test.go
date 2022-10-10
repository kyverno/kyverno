package utils

import (
	"testing"

	"gotest.tools/assert"
)

func Test_OriginalMapMustNotBeChanged(t *testing.T) {
	// no variables
	originalMap := map[string]interface{}{
		"rsc": 3711,
		"r":   2138,
		"gri": 1908,
		"adg": 912,
	}

	mapCopy := CopyMap(originalMap)
	mapCopy["r"] = 1

	assert.Equal(t, originalMap["r"], 2138)
}

func Test_OriginalSliceMustNotBeChanged(t *testing.T) {
	// no variables
	originalSlice := []interface{}{
		3711,
		2138,
		1908,
		912,
	}

	sliceCopy := CopySlice(originalSlice)
	sliceCopy[0] = 1

	assert.Equal(t, originalSlice[0], 3711)
}

func Test_allEmpty(t *testing.T) {
	var list []string
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyList(t *testing.T) {
	var list []string
	element := "foo"
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElement(t *testing.T) {
	list := []string{"foo", "bar"}
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == false)
}

func Test_emptyElementInList(t *testing.T) {
	list := []string{"foo", "bar", ""}
	var element string
	res := ContainsString(list, element)
	assert.Assert(t, res == true)

	list = []string{"foo", "bar", "bar"}
	element = "bar"
	res = ContainsString(list, element)
	assert.Assert(t, res == true)
}

func Test_containsNs(t *testing.T) {
	var patterns []string
	var res bool
	patterns = []string{"*"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"*", "default"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"default2", "default"}
	res = ContainsNamepace(patterns, "default1")
	assert.Assert(t, res == false)

	patterns = []string{"d*"}
	res = ContainsNamepace(patterns, "default")
	assert.Assert(t, res == true)

	patterns = []string{"d*"}
	res = ContainsNamepace(patterns, "test")
	assert.Assert(t, res == false)

	patterns = []string{}
	res = ContainsNamepace(patterns, "test")
	assert.Assert(t, res == false)
}

func Test_higherVersion(t *testing.T) {
	v, err := isVersionHigher("invalid.version", 1, 1, 1)
	assert.Assert(t, v == false && err != nil)

	v, err = isVersionHigher("invalid-version", 0, 0, 0)
	assert.Assert(t, v == false && err != nil)

	v, err = isVersionHigher("v1.1.1", 1, 1, 1)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.0.0", 1, 1, 1)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.5.9", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9+distro", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9+distro", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9-rc2", 1, 5, 9)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v1.5.9", 2, 1, 0)
	assert.Assert(t, v == false && err == nil)

	v, err = isVersionHigher("v2.1.0", 1, 5, 9)
	assert.Assert(t, v == true && err == nil)

	v, err = isVersionHigher("v1.5.9-x-v1.5.9.x", 1, 5, 8)
	assert.Assert(t, v == true && err == nil)
}

func Test_ConvertResource(t *testing.T) {
	testCases := []struct {
		name                 string
		raw                  string
		group, version, kind string
		namespace            string
		expectedNamespace    string
	}{
		{
			name:              "test-namespaced-resource-secret-with-namespace",
			raw:               `{"apiVersion": "v1","data": {"password": "YXNkO2xma2o4OTJsIC1uCg=="},"kind": "Secret","metadata": {"name": "my-secret","namespace": "test"},"type": "Opaque"}`,
			group:             "",
			version:           "v1",
			kind:              "Secret",
			namespace:         "mynamespace",
			expectedNamespace: "mynamespace",
		},
		{
			name:              "test-namespaced-resource-secret-without-namespace",
			raw:               `{"apiVersion": "v1","data": {"password": "YXNkO2xma2o4OTJsIC1uCg=="},"kind": "Secret","metadata": {"name": "my-secret"},"type": "Opaque"}`,
			group:             "",
			version:           "v1",
			kind:              "Secret",
			namespace:         "mynamespace",
			expectedNamespace: "mynamespace",
		},
		{
			name:              "test-cluster-resource-namespace-with-namespace",
			raw:               `{"apiVersion": "v1","kind": "Namespace","metadata": {"name": "my-namespace","namespace": "oldnamespace"},"type": "Opaque"}`,
			group:             "",
			version:           "v1",
			kind:              "Namespace",
			namespace:         "newnamespace",
			expectedNamespace: "",
		},
		{
			name:              "test-cluster-resource-namespace-without-namespace",
			raw:               `{"apiVersion": "v1","kind": "Namespace","metadata": {"name": "my-namespace"},"type": "Opaque"}`,
			group:             "",
			version:           "v1",
			kind:              "Namespace",
			namespace:         "newnamespace",
			expectedNamespace: "",
		},
		{
			name:              "test-cluster-resource-cluster-role-with-namespace",
			raw:               `{"apiVersion": "rbac.authorization.k8s.io/v1","kind": "ClusterRole","metadata": {"name": "my-cluster-role","namespace":"test"},"rules": [{"apiGroups": ["*"],"resources": ["namespaces"],"verbs": ["watch"]}]}`,
			group:             "rbac.authorization.k8s.io",
			version:           "v1",
			kind:              "ClusterRole",
			namespace:         "",
			expectedNamespace: "",
		},
		{
			name:              "test-cluster-resource-cluster-role-without-namespace",
			raw:               `{"apiVersion": "rbac.authorization.k8s.io/v1","kind": "ClusterRole","metadata": {"name": "my-cluster-role"},"rules": [{"apiGroups": ["*"],"resources": ["namespaces"],"verbs": ["watch"]}]}`,
			group:             "rbac.authorization.k8s.io",
			version:           "v1",
			kind:              "ClusterRole",
			namespace:         "",
			expectedNamespace: "",
		},
	}

	for _, test := range testCases {
		resource, err := ConvertResource([]byte(test.raw), test.group, test.version, test.kind, test.namespace)
		assert.NilError(t, err)
		assert.Assert(t, resource.GetNamespace() == test.expectedNamespace)
		break
	}
}

func Test_SeperateWildcards(t *testing.T) {
	testcases := []struct {
		description string
		inputList   []string
		expList1    []string
		expList2    []string
	}{
		{
			description: "tc1",
			inputList:   []string{"test*", "default", "default1", "hello"},
			expList1:    []string{"test*"},
			expList2:    []string{"default", "default1", "hello"},
		},
		{
			description: "tc2",
			inputList:   []string{"test*", "default*", "default1?", "hello?"},
			expList1:    []string{"test*", "default*", "default1?", "hello?"},
			expList2:    nil,
		},
		{
			description: "tc3",
			inputList:   []string{"test", "default", "default1", "hello"},
			expList1:    nil,
			expList2:    []string{"test", "default", "default1", "hello"},
		},
		{
			description: "tc4",
			inputList:   nil,
			expList1:    nil,
			expList2:    nil,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			list1, list2 := SeperateWildcards(tc.inputList)
			assert.DeepEqual(t, list1, tc.expList1)
			assert.DeepEqual(t, list2, tc.expList2)
		})
	}
}

func Test_CheckWildcardNamespaces(t *testing.T) {
	testcases := []struct {
		description   string
		inputPatterns []string
		inputNs       []string
		expString1    string
		expString2    string
		expBool       bool
	}{
		{
			description:   "tc1",
			inputPatterns: []string{"default*", "test*"},
			inputNs:       []string{"default", "default1"},
			expString1:    "default*",
			expString2:    "default",
			expBool:       true,
		},
		{
			description:   "tc2",
			inputPatterns: []string{"test*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "test*",
			expString2:    "test",
			expBool:       true,
		},
		{
			description:   "tc3",
			inputPatterns: []string{"*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "*",
			expString2:    "default1",
			expBool:       true,
		},
		{
			description:   "tc4",
			inputPatterns: []string{"a*"},
			inputNs:       []string{"default1", "test"},
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc5",
			inputPatterns: nil,
			inputNs:       []string{"default1", "test"},
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc6",
			inputPatterns: []string{"*"},
			inputNs:       nil,
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
		{
			description:   "tc7",
			inputPatterns: nil,
			inputNs:       nil,
			expString1:    "",
			expString2:    "",
			expBool:       false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			str1, str2, actualBool := CheckWildcardNamespaces(tc.inputPatterns, tc.inputNs)
			assert.Equal(t, str1, tc.expString1)
			assert.Equal(t, str2, tc.expString2)
			assert.Equal(t, actualBool, tc.expBool)
		})
	}
}

func Test_containsNamespaceWithStringReturn(t *testing.T) {
	testcases := []struct {
		description  string
		inputPattern []string
		inputNs      string
		expStr1      string
		expStr2      string
		expBool      bool
	}{
		{
			description:  "tc1",
			inputPattern: []string{"default*"},
			inputNs:      "default",
			expStr1:      "default*",
			expStr2:      "default",
			expBool:      true,
		},
		{
			description:  "tc2",
			inputPattern: []string{"*"},
			inputNs:      "default",
			expStr1:      "*",
			expStr2:      "default",
			expBool:      true,
		},
		{
			description:  "tc3",
			inputPattern: []string{"*"},
			inputNs:      "default",
			expStr1:      "*",
			expStr2:      "default",
			expBool:      true,
		},
		{
			description:  "tc4",
			inputPattern: nil,
			inputNs:      "default",
			expStr1:      "",
			expStr2:      "",
			expBool:      false,
		},
		{
			description:  "tc5",
			inputPattern: nil,
			inputNs:      "",
			expStr1:      "",
			expStr2:      "",
			expBool:      false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.description, func(t *testing.T) {
			str1, str2, actualBool := containsNamespaceWithStringReturn(tc.inputPattern, tc.inputNs)
			assert.Equal(t, str1, tc.expStr1)
			assert.Equal(t, str2, tc.expStr2)
			assert.Equal(t, actualBool, tc.expBool)
		})
	}
}
