package utils

import (
	"testing"

	"gotest.tools/assert"
)

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
