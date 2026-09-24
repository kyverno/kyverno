package libs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestValidatePathSegment(t *testing.T) {
	cases := []struct {
		value   string
		wantErr bool
	}{
		{"pods", false},
		{"my.app-name", false},
		{"", true},
		{".", true},
		{"..", true},
		{"pods/status", true},
		{"pods\\status", true},
	}
	for _, tc := range cases {
		err := validatePathSegment("resource", tc.value)
		if tc.wantErr {
			assert.Error(t, err, "value %q", tc.value)
		} else {
			assert.NoError(t, err, "value %q", tc.value)
		}
	}
}

func TestValidateGroupVersion(t *testing.T) {
	cases := []struct {
		name    string
		gv      schema.GroupVersion
		wantErr bool
	}{
		{"core group v1", schema.GroupVersion{Group: "", Version: "v1"}, false},
		{"named group", schema.GroupVersion{Group: "apps", Version: "v1"}, false},
		{"traversal group", schema.GroupVersion{Group: "..", Version: "v1"}, true},
		{"traversal version", schema.GroupVersion{Group: "apps", Version: ".."}, true},
		{"empty version", schema.GroupVersion{Group: "apps", Version: ""}, true},
	}
	for _, tc := range cases {
		err := validateGroupVersion(tc.gv)
		if tc.wantErr {
			assert.Error(t, err, tc.name)
		} else {
			assert.NoError(t, err, tc.name)
		}
	}
}

func TestValidateNamespaceSegment(t *testing.T) {
	assert.NoError(t, validateNamespaceSegment(""), "empty namespace is cluster-scoped and always allowed")
	assert.NoError(t, validateNamespaceSegment("default"))
	assert.Error(t, validateNamespaceSegment(".."))
	assert.Error(t, validateNamespaceSegment("."))
}

func TestSplitResourcePath(t *testing.T) {
	resource, sub, err := splitResourcePath("pods")
	assert.NoError(t, err)
	assert.Equal(t, "pods", resource)
	assert.Empty(t, sub)

	resource, sub, err = splitResourcePath("pods/status")
	assert.NoError(t, err)
	assert.Equal(t, "pods", resource)
	assert.Equal(t, []string{"status"}, sub)

	// GHSA-4pqm-wqq6-h9q2: a traversal segment hidden in the subresource
	// position must be rejected before it ever reaches the dynamic client.
	_, _, err = splitResourcePath("pods/../../secrets")
	assert.Error(t, err)

	_, _, err = splitResourcePath("./pods")
	assert.Error(t, err)

	_, _, err = splitResourcePath("pods/")
	assert.Error(t, err, "trailing slash produces an empty subresource segment")
}
