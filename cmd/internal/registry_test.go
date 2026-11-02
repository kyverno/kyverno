package internal

import "testing"

func TestValidateRegistryClientConfig(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		allowlist string
		mode      string
		wantError bool
	}{
		{name: "defaults"},
		{name: "audit", mode: "audit"},
		{name: "enforce", mode: "enforce", allowlist: "registry.internal,10.0.0.0/8,fd00::/8"},
		{name: "invalid mode", mode: "disabled", wantError: true},
		{name: "wildcard", allowlist: "*.internal", wantError: true},
		{name: "URL", allowlist: "https://registry.internal", wantError: true},
		{name: "invalid CIDR", allowlist: "10.0.0.0/33", wantError: true},
		{name: "empty list element", allowlist: "registry.internal,,10.0.0.1", wantError: true},
		{name: "trailing comma", allowlist: "registry.internal,", wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateRegistryClientConfig(tc.allowlist, tc.mode)
			if (err != nil) != tc.wantError {
				t.Fatalf("validateRegistryClientConfig() error = %v, want error = %v", err, tc.wantError)
			}
		})
	}
}
