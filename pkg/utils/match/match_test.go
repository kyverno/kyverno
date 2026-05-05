package match

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCheckNamespace_EmptyStatement(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("default")

	err := CheckNamespace("", resource)
	assert.NoError(t, err, "empty statement should match any namespace")
}

func TestCheckNamespace_MatchingNamespace(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("kube-system")

	err := CheckNamespace("kube-system", resource)
	assert.NoError(t, err, "matching namespace should not return error")
}

func TestCheckNamespace_NonMatchingNamespace(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("default")

	err := CheckNamespace("kube-system", resource)
	assert.Error(t, err, "non-matching namespace should return error")
	assert.Contains(t, err.Error(), "default")
	assert.Contains(t, err.Error(), "kube-system")
}

func TestCheckNamespace_EmptyResourceNamespace(t *testing.T) {
	resource := unstructured.Unstructured{}
	// No namespace set (cluster-scoped resource)

	err := CheckNamespace("default", resource)
	assert.Error(t, err, "empty resource namespace should not match non-empty statement")
}

func TestCheckNameSpace_EmptyNamespaces(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("default")

	result := CheckNameSpace([]string{}, resource)
	assert.False(t, result, "empty namespaces slice should return false")
}

func TestCheckNameSpace_MatchingNamespace(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("production")

	result := CheckNameSpace([]string{"staging", "production", "development"}, resource)
	assert.True(t, result, "should match when namespace is in the list")
}

func TestCheckNameSpace_NonMatchingNamespace(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("test")

	result := CheckNameSpace([]string{"staging", "production"}, resource)
	assert.False(t, result, "should not match when namespace is not in list")
}

func TestCheckNameSpace_WildcardMatch(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("team-alpha-prod")

	result := CheckNameSpace([]string{"team-*-prod"}, resource)
	assert.True(t, result, "wildcard pattern should match")
}

func TestCheckNameSpace_WildcardNoMatch(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("team-alpha-dev")

	result := CheckNameSpace([]string{"team-*-prod"}, resource)
	assert.False(t, result, "wildcard pattern should not match different suffix")
}

func TestCheckNameSpace_NamespaceKind(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetKind("Namespace")
	resource.SetName("kube-system")

	result := CheckNameSpace([]string{"kube-system"}, resource)
	assert.True(t, result, "Namespace kind should match by name, not namespace field")
}

func TestCheckNameSpace_NamespaceKindNoMatch(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetKind("Namespace")
	resource.SetName("default")

	result := CheckNameSpace([]string{"kube-system"}, resource)
	assert.False(t, result, "Namespace kind should not match when name differs")
}

func TestCheckNameSpace_WildcardAll(t *testing.T) {
	resource := unstructured.Unstructured{}
	resource.SetNamespace("any-namespace")

	result := CheckNameSpace([]string{"*"}, resource)
	assert.True(t, result, "* wildcard should match any namespace")
}

func TestCheckNameSpace_MultiplePatterns(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
		patterns  []string
		expected  bool
	}{
		{
			name:      "first pattern matches",
			namespace: "staging",
			patterns:  []string{"staging", "prod-*"},
			expected:  true,
		},
		{
			name:      "second pattern matches",
			namespace: "prod-us-east",
			patterns:  []string{"staging", "prod-*"},
			expected:  true,
		},
		{
			name:      "no pattern matches",
			namespace: "development",
			patterns:  []string{"staging", "prod-*"},
			expected:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := unstructured.Unstructured{}
			resource.SetNamespace(tc.namespace)

			result := CheckNameSpace(tc.patterns, resource)
			assert.Equal(t, tc.expected, result)
		})
	}
}
