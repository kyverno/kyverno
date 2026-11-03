package egress

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// The two denylists are the entire policy: nothing is denied that is not on one of them, and
// nothing survives underneath when they are replaced. That makes the nil/empty distinction load
// bearing in both directions -- read an operator's cleared list as "unset" and they stay blocked
// forever, read a caller's zero value as "cleared" and every install without a ConfigMap entry
// silently drops its defaults.

func TestConfiguredDenylistsReplaceTheDefaults(t *testing.T) {
	t.Parallel()
	// 198.18.0.0/15 is a shipped default; 8.8.0.0/16 is not. Configuring only the latter must
	// swap which of the two is denied, not append to the shipped list.
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedIPs: []string{"8.8.0.0/16"}}, map[string][]string{
		"benchmark.example": {"198.18.0.1"},
		"custom.example":    {"8.8.4.4"},
	})
	_, err := policy.ResolveAndValidate(context.Background(), "custom.example")
	require.ErrorIs(t, err, ErrAddressBlocked, "a configured range must be denied")
	_, err = policy.ResolveAndValidate(context.Background(), "benchmark.example")
	require.NoError(t, err, "a default dropped from the configured list must stop being denied")

	// The host list behaves the same way, and the two lists are independent: replacing one must
	// leave the other at its defaults rather than clearing it too.
	hosts := testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{"blocked.example"}}, map[string][]string{
		"blocked.example":          {publicIPv4},
		"metadata.google.internal": {publicIPv4},
		"documentation.example":    {"203.0.113.1"},
	})
	_, err = hosts.ResolveAndValidate(context.Background(), "blocked.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
	_, err = hosts.ResolveAndValidate(context.Background(), "metadata.google.internal")
	require.NoError(t, err, "a shipped metadata name dropped from the configured host list must stop being denied")
	_, err = hosts.ResolveAndValidate(context.Background(), "documentation.example")
	require.ErrorIs(t, err, ErrAddressBlocked, "replacing the host list must not disturb the address list")
}

func TestClearedDenylistsDenyNothing(t *testing.T) {
	t.Parallel()
	// A non-nil empty slice is an operator clearing the list, which New must honor rather than
	// read as "unset" and reinstate the defaults. Nothing is held back from the clear: there is
	// no hard-blocked floor under these lists any more, so loopback and metadata go with the
	// rest. An operator who clears the list has to mean it, and the chart is where they are
	// told what they are giving up.
	answers := map[string][]string{
		"loopback.example":         {"127.0.0.1"},
		"metadata.example":         {"169.254.169.254"},
		"rfc1918.example":          {"10.0.0.1"},
		"ula.example":              {"fd00::1"},
		"metadata.google.internal": {publicIPv4},
	}
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{}, DeniedIPs: []string{}}, answers)
	for host := range answers {
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.NoError(t, err, "%s stayed denied after both lists were cleared", host)
	}
}

func TestNilDenylistsKeepTheDefaults(t *testing.T) {
	t.Parallel()
	// Callers that know nothing about this setting -- the CLI, a ConfigMap with no entry for it,
	// and most tests in this package -- leave both fields at their zero value and must still get
	// the shipped rules.
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{
		"documentation.example":    {"203.0.113.1"},
		"metadata.google.internal": {publicIPv4},
	})
	_, err := policy.ResolveAndValidate(context.Background(), "documentation.example")
	require.ErrorIs(t, err, ErrAddressBlocked, "a nil address list must keep DefaultDeniedIPs")
	_, err = policy.ResolveAndValidate(context.Background(), "metadata.google.internal")
	require.ErrorIs(t, err, ErrAddressBlocked, "a nil host list must keep DefaultDeniedHosts")
}

func TestIPv4MappedDenyPrefixIsRewritten(t *testing.T) {
	t.Parallel()
	// Matching runs on unmapped candidates, so an IPv4-mapped prefix has to be rewritten to its
	// IPv4 form at construction or it silently matches nothing an operator can reach.
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedIPs: []string{"::ffff:8.8.0.0/112"}}, map[string][]string{
		"mapped.example":  {"8.8.4.4"},
		"outside.example": {"8.9.0.1"},
	})
	_, err := policy.ResolveAndValidate(context.Background(), "mapped.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
	_, err = policy.ResolveAndValidate(context.Background(), "outside.example")
	require.NoError(t, err, "the rewrite dropped 96 bits of prefix length as well as 96 bits of address")
}

func TestIPv4MappedDenyPrefixShorterThanSlash96IsSkipped(t *testing.T) {
	t.Parallel()
	// ::ffff:10.0.0.0/64 covers address space outside the mapped range, so there is no IPv4
	// prefix it can be rewritten to. Taking it literally would deny all of IPv4 to an operator
	// who meant one subnet, so it is dropped -- and the entry beside it still applies, which is
	// what makes the drop show up as one missing range instead of an empty policy.
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedIPs: []string{"::ffff:10.0.0.0/64", "8.8.0.0/16"}}, map[string][]string{
		"wide.example":   {"10.0.0.1"},
		"public.example": {publicIPv4},
		"other.example":  {"8.8.4.4"},
	})
	for _, host := range []string{"wide.example", "public.example"} {
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.NoError(t, err, "the skipped entry denied %s", host)
	}
	_, err := policy.ResolveAndValidate(context.Background(), "other.example")
	require.ErrorIs(t, err, ErrAddressBlocked, "a skipped entry discarded the rest of the list")
}

func TestInvalidDeniedIPEntriesAreSkipped(t *testing.T) {
	t.Parallel()
	// These lists come from a ConfigMap an operator edits at runtime. A typo used to fail New
	// and take the admission controller's webhook down with it, so it is now logged and skipped.
	// The regression that replaces it is the opposite one: a bad entry must not swallow the rest
	// of the list, and must not deny anything itself.
	for _, entry := range []string{
		"", " ", "10.0.0.0/99", "10.0.0.999", "010.0.0.1", "registry.internal",
		"10.0.0.0/8,10.1.0.0/16", "fe80::1%eth0", "::ffff:10.0.0.0/64", "10.0.0.1:443",
	} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedIPs: []string{entry, "8.8.0.0/16"}}, map[string][]string{
				"good.example":    {"8.8.4.4"},
				"public.example":  {publicIPv4},
				"rfc1918.example": {"10.0.0.1"},
			})
			require.Len(t, policy.prefixes, 1, "the invalid entry was kept")
			_, err := policy.ResolveAndValidate(context.Background(), "good.example")
			require.ErrorIs(t, err, ErrAddressBlocked, "a bad entry discarded the valid one beside it")
			for _, host := range []string{"public.example", "rfc1918.example"} {
				_, err := policy.ResolveAndValidate(context.Background(), host)
				require.NoError(t, err, "the invalid entry denied %s", host)
			}
		})
	}
}

func TestInvalidDeniedHostEntriesAreSkipped(t *testing.T) {
	t.Parallel()
	// Same contract on the host side, with two extra reasons to drop an entry: a port can never
	// be matched against a bare destination host, and an IP literal would never be compared
	// against this list at all because addresses are checked against the other one.
	for _, entry := range []string{
		"", " ", "*.internal", "https://registry.internal", "registry.internal/path",
		"user@registry.internal", "registry..internal", "registry:bad", "registry:65536",
		"registry:", "-registry.example", "registry.internal:5000", "[fd01::1]:443",
		"10.0.0.1:443", "10.0.0.1", "fd01::1", "fe80::1%eth0",
	} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{entry, "good.example"}, DeniedIPs: []string{}}, map[string][]string{
				"good.example":      {publicIPv4},
				"registry.internal": {publicIPv4},
			})
			require.Len(t, policy.hosts, 1, "the invalid entry was kept")
			_, err := policy.ResolveAndValidate(context.Background(), "good.example")
			require.ErrorIs(t, err, ErrAddressBlocked, "a bad entry discarded the valid one beside it")
			_, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.NoError(t, err, "the invalid entry denied registry.internal")
		})
	}
}
