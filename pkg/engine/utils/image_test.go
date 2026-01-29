package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ImageMatches_exact_match(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx:latest"}

	got := ImageMatches("nginx:latest", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_wildcard_tag(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx:*"}

	got := ImageMatches("nginx:1.21", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_wildcard_registry(t *testing.T) {
	t.Parallel()
	patterns := []string{"gcr.io/my-project/*"}

	got := ImageMatches("gcr.io/my-project/app:v1", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_no_match(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx:*", "redis:*"}

	got := ImageMatches("postgres:15", patterns)

	assert.False(t, got)
}

func Test_ImageMatches_empty_patterns(t *testing.T) {
	t.Parallel()
	patterns := []string{}

	got := ImageMatches("nginx:latest", patterns)

	assert.False(t, got)
}

func Test_ImageMatches_multiple_patterns_first_match(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx:*", "redis:*", "postgres:*"}

	got := ImageMatches("nginx:alpine", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_multiple_patterns_last_match(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx:*", "redis:*", "postgres:*"}

	got := ImageMatches("postgres:14", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_full_registry_path(t *testing.T) {
	t.Parallel()
	patterns := []string{"docker.io/library/nginx:*"}

	got := ImageMatches("docker.io/library/nginx:1.25", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_digest(t *testing.T) {
	t.Parallel()
	patterns := []string{"nginx@sha256:*"}

	got := ImageMatches("nginx@sha256:abc123def456", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_partial_name_no_match(t *testing.T) {
	t.Parallel()
	// should not match partial names without wildcard
	patterns := []string{"nginx"}

	got := ImageMatches("nginx:latest", patterns)

	assert.False(t, got)
}

func Test_ImageMatches_double_wildcard(t *testing.T) {
	t.Parallel()
	patterns := []string{"*/*:*"}

	got := ImageMatches("myrepo/myimage:v1", patterns)

	assert.True(t, got)
}

func Test_ImageMatches_star_matches_anything(t *testing.T) {
	t.Parallel()
	patterns := []string{"*"}

	got := ImageMatches("literally-anything", patterns)

	assert.True(t, got)
}
