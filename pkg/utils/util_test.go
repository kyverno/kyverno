package utils

import (
	"testing"

	"gotest.tools/assert"
)

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
