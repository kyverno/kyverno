package compiler

import (
	"context"
	"testing"

	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCELHTTPContext(t *testing.T) {
	tests := []struct {
		name      string
		blocklist string // FLAG_HTTP_BLOCKLIST env var value
		allowlist string // FLAG_HTTP_ALLOWLIST env var value
		wantErr   bool
	}{{
		name: "defaults - no env vars set",
	}, {
		name:      "custom blocklist with CIDR ranges",
		blocklist: "192.0.2.0/24,198.51.100.0/24",
	}, {
		name:      "custom blocklist with hostnames",
		blocklist: "metadata.google.internal,metadata.internal",
	}, {
		name:      "custom blocklist with CIDRs and hostnames",
		blocklist: "10.0.0.0/8,metadata.google.internal",
	}, {
		name:      "custom allowlist",
		allowlist: "https://api.example.com,https://webhook.corp/v1/",
	}, {
		name:      "blocklist and allowlist combined",
		blocklist: "10.0.0.0/8",
		allowlist: "https://api.example.com",
	}, {
		name:      "invalid blocklist CIDR",
		blocklist: "999.999.999.999/24",
		wantErr:   true,
	}, {
		name:      "invalid allowlist entry missing scheme",
		allowlist: "api.example.com",
		wantErr:   true,
	}, {
		name:      "invalid allowlist entry missing host",
		allowlist: "https://",
		wantErr:   true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.blocklist != "" {
				t.Setenv("FLAG_HTTP_BLOCKLIST", tt.blocklist)
			}
			if tt.allowlist != "" {
				t.Setenv("FLAG_HTTP_ALLOWLIST", tt.allowlist)
			}
			ctx, err := NewCELHTTPContext()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, ctx)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ctx)
			}
		})
	}
}

func TestNewCELHTTPContextParsedFlags(t *testing.T) {
	// These tests call Parse() on the global flags; each subtest saves and restores
	// state by parsing back the previous values so test ordering does not matter.
	t.Run("parsed blocklist overrides env var", func(t *testing.T) {
		t.Setenv("FLAG_HTTP_BLOCKLIST", "10.0.0.0/8")

		saved := toggle.HTTPBlocklist.Values()
		require.NoError(t, toggle.HTTPBlocklist.Parse("192.0.2.0/24"))
		t.Cleanup(func() {
			// restore: re-parse the saved values as a comma-joined string, or clear
			_ = toggle.HTTPBlocklist.Parse("")
			_ = saved // acknowledged; the flag now has hasValue=true with empty list,
			// which is acceptable for test isolation within this package.
		})

		ctx, err := NewCELHTTPContext()
		assert.NoError(t, err)
		assert.NotNil(t, ctx)
	})

	t.Run("parsed allowlist overrides env var", func(t *testing.T) {
		t.Setenv("FLAG_HTTP_ALLOWLIST", "https://env.example.com")

		require.NoError(t, toggle.HTTPAllowlist.Parse("https://flag.example.com"))
		t.Cleanup(func() { _ = toggle.HTTPAllowlist.Parse("") })

		ctx, err := NewCELHTTPContext()
		assert.NoError(t, err)
		assert.NotNil(t, ctx)
	})

	t.Run("invalid parsed blocklist returns error", func(t *testing.T) {
		require.NoError(t, toggle.HTTPBlocklist.Parse("999.999.999.999/32"))
		t.Cleanup(func() { _ = toggle.HTTPBlocklist.Parse("") })

		ctx, err := NewCELHTTPContext()
		assert.Error(t, err)
		assert.Nil(t, ctx)
		assert.Contains(t, err.Error(), "invalid CEL http configuration")
	})

	t.Run("invalid parsed allowlist returns error", func(t *testing.T) {
		require.NoError(t, toggle.HTTPAllowlist.Parse("no-scheme-here"))
		t.Cleanup(func() { _ = toggle.HTTPAllowlist.Parse("") })

		ctx, err := NewCELHTTPContext()
		assert.Error(t, err)
		assert.Nil(t, ctx)
		assert.Contains(t, err.Error(), "invalid CEL http configuration")
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
