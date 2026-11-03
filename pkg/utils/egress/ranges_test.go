package egress

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The private-class list is operator-configurable because the shipped defaults are a judgement
// call about what a cluster's local infrastructure looks like, and a cluster that genuinely
// routes one of those ranges should not have to allowlist it forever. These tests pin what the
// configuration can and cannot reach: it replaces the defaults wholesale, it can be cleared, and
// clearing it never reaches the hard blocks or Go's own private classification.
func TestConfiguredPrivateRangesReplaceTheDefaults(t *testing.T) {
	t.Parallel()
	// 198.18.0.0/15 is a default; 8.8.0.0/16 is not. Configuring only the latter must swap
	// which of the two needs an exemption, not add to the list.
	answers := map[string][]string{"benchmark.example": {"198.18.0.1"}, "custom.example": {"8.8.4.4"}}
	policy := testPolicy(t, Config{Mode: ModeEnforce, PrivateRanges: []string{"8.8.0.0/16"}}, answers)
	_, err := policy.ResolveAndValidate(context.Background(), "custom.example")
	require.ErrorIs(t, err, ErrAddressBlocked, "a configured range must be private-class")
	_, err = policy.ResolveAndValidate(context.Background(), "benchmark.example")
	require.NoError(t, err, "a default dropped from the configured list must become ordinary public space")
}

func TestClearedPrivateRangesKeepTheFloor(t *testing.T) {
	t.Parallel()
	// A non-nil empty slice is an operator clearing the list, which New must honor rather than
	// read as "unset" and reinstate the defaults.
	answers := map[string][]string{
		"documentation.example": {"203.0.113.1"},
		"rfc1918.example":       {"10.0.0.1"},
		"ula.example":           {"fd00::1"},
		"metadata.example":      {"169.254.169.254"},
	}
	policy := testPolicy(t, Config{Mode: ModeEnforce, PrivateRanges: []string{}}, answers)
	_, err := policy.ResolveAndValidate(context.Background(), "documentation.example")
	require.NoError(t, err, "clearing the list must drop its ranges")
	for _, host := range []string{"rfc1918.example", "ula.example", "metadata.example"} {
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.ErrorIs(t, err, ErrAddressBlocked, "%s is not part of the configurable list", host)
	}
}

func TestNilPrivateRangesKeepTheDefaults(t *testing.T) {
	t.Parallel()
	// Callers that know nothing about this setting -- the CLI, and every other test in this
	// package -- leave the field at its zero value and must still get the shipped list.
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"documentation.example": {"203.0.113.1"}})
	_, err := policy.ResolveAndValidate(context.Background(), "documentation.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
}

func TestPrivateRangeValidation(t *testing.T) {
	t.Parallel()
	// A bad range fails construction, so a controller exits at startup instead of running with
	// a range the operator believes is covered.
	for _, entry := range []string{"", " ", "10.0.0.0/99", "10.0.0.999", "registry.internal", "10.0.0.0/8,10.1.0.0/16", "fe80::1%eth0", "::ffff:10.0.0.0/64"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			_, err := New(Config{PrivateRanges: []string{entry}})
			require.Error(t, err)
		})
	}
	// An IPv4-mapped prefix at or below /96 is rewritten to its IPv4 form, because validation
	// runs on unmapped candidates.
	policy := testPolicy(t, Config{Mode: ModeEnforce, PrivateRanges: []string{"::ffff:8.8.0.0/112"}}, map[string][]string{"mapped.example": {"8.8.4.4"}})
	_, err := policy.ResolveAndValidate(context.Background(), "mapped.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
}

func TestParseRangesIsNeverNil(t *testing.T) {
	t.Parallel()
	// ParseRanges and ParseAllowlist differ deliberately on the empty string: nil is how
	// Config.PrivateRanges spells "keep the defaults", so the flag parser must not produce it
	// for an operator who rendered an empty list.
	require.Equal(t, []string{}, ParseRanges(""))
	require.Equal(t, []string{}, ParseRanges("  "))
	require.Nil(t, ParseAllowlist(""))
	require.Equal(t, []string{"10.0.0.0/8", "fd00::/8"}, ParseRanges(" 10.0.0.0/8 ,, fd00::/8 "))
}
