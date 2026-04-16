package compiler

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLazyCELHTTPContext_NeverErrors(t *testing.T) {
	// Construction must never error even with a completely invalid blocklist,
	// because the lazy context reads flags at call time, not construction time.
	t.Run("invalid blocklist does not error at construction", func(t *testing.T) {
		require.NoError(t, toggle.HTTPBlocklist.Parse("999.999.999.999/24"))
		t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

		ctx := NewLazyCELHTTPContext("")
		assert.NotNil(t, ctx)
	})

	t.Run("invalid allowlist does not error at construction", func(t *testing.T) {
		require.NoError(t, toggle.HTTPAllowlist.Parse("no-scheme-here"))
		t.Cleanup(func() { toggle.HTTPAllowlist.Reset() })

		ctx := NewLazyCELHTTPContext("test-namespace")
		assert.NotNil(t, ctx)
	})
}

func TestNewLazyCELHTTPContext_ToggleEnforcement(t *testing.T) {
	t.Run("namespaced context blocks http.Get when toggle is off", func(t *testing.T) {
		// toggle is off by default
		ctx := NewLazyCELHTTPContext("test-namespace")
		require.NotNil(t, ctx)

		_, err := ctx.Get("http://example.com", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed in namespaced policies")
	})

	t.Run("namespaced context blocks http.Post when toggle is off", func(t *testing.T) {
		ctx := NewLazyCELHTTPContext("test-namespace")
		require.NotNil(t, ctx)

		_, err := ctx.Post("http://example.com", nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed in namespaced policies")
	})

	t.Run("namespaced Client() inherits toggle enforcement", func(t *testing.T) {
		ctx := NewLazyCELHTTPContext("test-namespace")
		require.NotNil(t, ctx)

		child, err := ctx.Client("")
		require.NoError(t, err)
		require.NotNil(t, child)

		_, err = child.Get("http://example.com", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not allowed in namespaced policies")
	})

	t.Run("namespaced context allows calls when toggle is on", func(t *testing.T) {
		t.Setenv("FLAG_ENABLE_HTTP_IN_NAMESPACED_POLICIES", "true")

		ctx := NewLazyCELHTTPContext("test-namespace")
		require.NotNil(t, ctx)

		// Will fail with a network/blocklist error (no real server) but NOT the toggle error.
		_, err := ctx.Get("http://example.com", nil)
		if err != nil {
			assert.NotContains(t, err.Error(), "not allowed in namespaced policies")
		}
	})

	t.Run("cluster-scoped context skips toggle check", func(t *testing.T) {
		// Empty namespace → no toggle wrapper; toggle off should not affect it.
		ctx := NewLazyCELHTTPContext("")
		require.NotNil(t, ctx)

		// Will fail with a network/blocklist error but NOT the toggle error.
		_, err := ctx.Get("http://example.com", nil)
		if err != nil {
			assert.NotContains(t, err.Error(), "not allowed in namespaced policies")
		}
	})
}

func TestAllowHTTPInNamespacedPoliciesToggle(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{{
		name: "default is false",
		want: false,
	}, {
		name: "enabled via env var",
		env:  "true",
		want: true,
	}, {
		name: "disabled via env var",
		env:  "false",
		want: false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv("FLAG_ENABLE_HTTP_IN_NAMESPACED_POLICIES", tt.env)
			}
			// Verify through the Toggles interface as the compilers would.
			got := toggle.FromContext(context.TODO()).AllowHTTPInNamespacedPolicies()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpressionsUseHTTP(t *testing.T) {
	tests := []struct {
		name        string
		expressions []string
		want        bool
	}{{
		name:        "empty list",
		expressions: nil,
		want:        false,
	}, {
		name:        "no http reference",
		expressions: []string{"object.metadata.name == 'foo'"},
		want:        false,
	}, {
		name:        "http.Get call",
		expressions: []string{"http.Get('https://example.com')"},
		want:        true,
	}, {
		name:        "http.Post in variables",
		expressions: []string{"object.spec.replicas > 1", "http.Post('https://example.com', {})"},
		want:        true,
	}, {
		name:        "string literal containing http — not an ident",
		expressions: []string{"'http.Get is a function'"},
		want:        false,
	}, {
		name:        "variable named httpClient — different ident",
		expressions: []string{"httpClient.Get('https://example.com')"},
		want:        false,
	}, {
		name:        "malformed expression is skipped",
		expressions: []string{"{{{{invalid", "http.Get('https://example.com')"},
		want:        true,
	}, {
		name:        "empty string is skipped",
		expressions: []string{"", "object.spec.name == 'test'"},
		want:        false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpressionsUseHTTP(tt.expressions...)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAllowHTTPInNamespacedPoliciesToggledViaParse(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{{
		name:  "parse true enables",
		value: "true",
		want:  true,
	}, {
		name:  "parse false disables",
		value: "false",
		want:  false,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, toggle.AllowHTTPInNamespacedPolicies.Parse(tt.value))
			t.Cleanup(func() { _ = toggle.AllowHTTPInNamespacedPolicies.Parse("false") })

			got := toggle.FromContext(context.TODO()).AllowHTTPInNamespacedPolicies()
			assert.Equal(t, tt.want, got)
		})
	}
}
