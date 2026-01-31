package dclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_match_both_empty_patterns(t *testing.T) {
	t.Parallel()
	// empty patterns should match everything
	got := match("", "", "default", "my-pod")

	assert.True(t, got)
}

func Test_match_name_pattern_only(t *testing.T) {
	t.Parallel()
	got := match("", "nginx-*", "default", "nginx-abc123")

	assert.True(t, got)
}

func Test_match_name_pattern_no_match(t *testing.T) {
	t.Parallel()
	got := match("", "nginx-*", "default", "redis-xyz")

	assert.False(t, got)
}

func Test_match_namespace_pattern_only(t *testing.T) {
	t.Parallel()
	got := match("prod-*", "", "prod-us", "any-pod")

	assert.True(t, got)
}

func Test_match_namespace_pattern_no_match(t *testing.T) {
	t.Parallel()
	got := match("prod-*", "", "staging", "any-pod")

	assert.False(t, got)
}

func Test_match_both_patterns(t *testing.T) {
	t.Parallel()
	got := match("default", "web-*", "default", "web-frontend")

	assert.True(t, got)
}

func Test_match_both_patterns_ns_fails(t *testing.T) {
	t.Parallel()
	// namespace doesn't match, should fail even if name matches
	got := match("prod", "web-*", "staging", "web-frontend")

	assert.False(t, got)
}

func Test_match_both_patterns_name_fails(t *testing.T) {
	t.Parallel()
	// name doesn't match, should fail even if namespace matches
	got := match("default", "web-*", "default", "api-backend")

	assert.False(t, got)
}

func Test_match_exact_namespace_wildcard_name(t *testing.T) {
	t.Parallel()
	got := match("kube-system", "*", "kube-system", "coredns-abc")

	assert.True(t, got)
}

func Test_match_wildcard_namespace_exact_name(t *testing.T) {
	t.Parallel()
	got := match("*", "nginx", "any-namespace", "nginx")

	assert.True(t, got)
}

func Test_match_double_wildcard(t *testing.T) {
	t.Parallel()
	got := match("*", "*", "anything", "something")

	assert.True(t, got)
}

func Test_match_cluster_scoped_resource(t *testing.T) {
	t.Parallel()
	// cluster-scoped resources have empty namespace
	got := match("", "my-ns-*", "", "my-ns-test")

	assert.True(t, got)
}

func Test_match_empty_name_with_namespace_pattern(t *testing.T) {
	t.Parallel()
	// namespace matches, name pattern is empty (matches any)
	got := match("test-*", "", "test-env", "whatever")

	assert.True(t, got)
}
