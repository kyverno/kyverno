package egress

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures standing in for an ordinary public registry or proxy address, shared by every test in
// this package. The RFC 5737/3849/9637 documentation ranges (192.0.2.0/24, 198.51.100.0/24,
// 203.0.113.0/24, 2001:db8::/32, 3fff::/20) cannot play that role: they are in DefaultDeniedIPs,
// so a fixture written there is logged in audit mode and rejected in enforce mode. Documentation
// addresses belong only in tests whose point is that they are denied. These are real public
// addresses, but nothing is ever contacted -- every resolver and dialer here is stubbed.
const (
	publicIPv4    = "93.184.216.34"
	publicIPv4Alt = "93.184.216.35"
	publicIPv6    = "2606:2800:220:1:248:1893:25c8:1946"
	// publicIPv4Nth formats distinct public addresses for tests that need a changing answer.
	publicIPv4Nth = "93.184.216.%d"
)

func testPolicy(t *testing.T, config Config, answers map[string][]string) *Policy {
	t.Helper()
	policy, err := New(config)
	require.NoError(t, err)
	policy.lookupNetIP = func(_ context.Context, _ string, host string) ([]netip.Addr, error) {
		values, ok := answers[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		addresses := make([]netip.Addr, len(values))
		for i, value := range values {
			addresses[i] = netip.MustParseAddr(value)
		}
		return addresses, nil
	}
	return policy
}

// One address per range in DefaultDeniedIPs. The list is the whole policy now -- there is no
// classification underneath it and no allowlist over it -- so a range dropped from defaults.go
// simply stops being denied, with nothing left to catch it. Each case also pins the audit/enforce
// split: audit connects anyway, because turning the guard on in a running cluster must only
// produce logs until the operator has seen what enforce would reject.
func TestDefaultDeniedIPsAreDenied(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"127.0.0.1", "::1", "0.0.0.0", "::", "169.254.169.254", "fe80::1", "224.0.0.1", "ff02::1",
		"10.1.2.3", "172.16.1.1", "192.168.1.1", "fd01::1", "100.100.100.200", "168.63.129.16", "fd00:ec2::254",
		"100.64.1.1", "198.18.0.1", "192.0.0.1", "192.88.99.1", "240.0.0.1", "255.255.255.255",
		"2001:0:4136:e378:8000:63bf:3fff:fdd2", "100::1",
		"192.0.2.1", "198.51.100.1", "203.0.113.1", "2001:db8::1", "3fff::1",
		// fec0::/10 is adjacent to fe80::/10, not inside it, so the link-local entry does not
		// cover either end of it; feff:ffff::1 is the top of the range.
		"fec0::1", "feff:ffff::1", "5f00::1", "64:ff9b:1::1",
	} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			answers := map[string][]string{"registry.internal": {address}}
			// The destination is spelled with mixed case and a trailing dot to pin that
			// normalization happens before anything is matched or resolved.
			audit := testPolicy(t, Config{}, answers)
			resolved, err := audit.ResolveAndValidate(context.Background(), "Registry.Internal.")
			require.NoError(t, err, "audit mode must log and connect")
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(address)}, resolved)
			enforce := testPolicy(t, Config{Mode: ModeEnforce}, answers)
			_, err = enforce.ResolveAndValidate(context.Background(), "registry.internal")
			require.ErrorIs(t, err, ErrAddressBlocked)
			assert.NotContains(t, err.Error(), address, "the resolved address leaked into the error")
		})
	}
}

// A CIDR cannot express that ::ffff:169.254.169.254, 64:ff9b::a9fe:a9fe and 2002:a9fe:a9fe:: all
// reach 169.254.169.254, so addressCandidates extracts the embedded IPv4 address and both forms
// are matched. Without it every IPv4 entry on the list is bypassable by spelling the destination
// as IPv6, which is not hypothetical: NAT64 is the normal form in an IPv6-only cluster.
func TestIPv6SpellingsOfADeniedIPv4AreDenied(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, address string }{
		{name: "IPv4-mapped metadata", address: "::ffff:169.254.169.254"},
		{name: "IPv4-compatible metadata", address: "::169.254.169.254"},
		{name: "NAT64 metadata", address: "64:ff9b::a9fe:a9fe"},
		{name: "6to4 metadata", address: "2002:a9fe:a9fe::"},
		{name: "IPv4-mapped RFC 1918", address: "::ffff:10.0.0.1"},
		{name: "NAT64 RFC 1918", address: "64:ff9b::a00:1"},
		{name: "IPv4-mapped protocol assignment", address: "::ffff:192.0.0.1"},
		{name: "NAT64 protocol assignment", address: "64:ff9b::c000:1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"registry.internal": {tc.address}})
			_, err := policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.ErrorIs(t, err, ErrAddressBlocked)
		})
	}
	// The original address is always kept as a candidate too, so an IPv6-only entry still
	// matches an address that carries no embedded IPv4 at all.
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedIPs: []string{"2606:2800::/32"}}, map[string][]string{"registry.example": {publicIPv6}})
	_, err := policy.ResolveAndValidate(context.Background(), "registry.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
}

// DefaultDeniedIPs is derived from the IANA special-purpose registries by taking every "Globally
// Reachable: False" entry. The entries marked True are real routed services and must stay
// reachable in enforce mode, or the guard breaks ordinary fetches while claiming to deny only
// local infrastructure. This is the other half of the derivation rule in defaults.go: that list
// says what to include, this test pins what deliberately stays out.
func TestGloballyReachableRangesAreNotDenied(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, address string }{
		{name: "AS112 v4", address: "192.31.196.1"},
		{name: "AS112 v4 direct delegation", address: "192.175.48.1"},
		{name: "AMT", address: "192.52.193.1"},
		{name: "AS112 v6 direct delegation", address: "2620:4f:8000::1"},
		// 6to4 carrying a public IPv4 is a public destination. 2002::/16 as a whole is not
		// denied; EmbeddedIPv4 unwraps it and the embedded address decides.
		{name: "6to4 wrapping a public address", address: "2002:5db8:d822::1"},
		// fec0::/10 sits directly above fe80::/10 and fc00::/7 directly below it. fe00::1 falls
		// between ULA and link-local, so it catches any of the three over-reaching downward.
		// The upper edge of fec0::/10 is pinned by the feff:ffff::1 case above.
		{name: "between the ULA and link local ranges", address: "fe00::1"},
		// 3fff::/20 ends at 3fff:0fff:..., well short of 3fff:ffff:..., and both sides of that
		// edge are inside the 2000::/3 global unicast pool. 5f00::/16 ends at 5f00:ffff:...
		{name: "above the RFC 9637 documentation range", address: "3fff:1000::1"},
		{name: "above the SRv6 range", address: "5f01::1"},
		// 64:ff9b:1::/48 is a /48, not the whole 64:ff9b::/32.
		{name: "beside the local use NAT64 range", address: "64:ff9b:2::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{"registry.example": {tc.address}})
			resolved, err := policy.ResolveAndValidate(context.Background(), "registry.example")
			require.NoError(t, err, "enforce mode denied a globally routed address")
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(tc.address)}, resolved)
		})
	}
}

// The metadata services are denied by name as well as by address because a name is what an image
// reference actually carries, and the address it resolves to is the provider's choice. Each name
// here resolves to an ordinary public address, so only the host list can catch it.
func TestDefaultDeniedHostsAreDeniedByName(t *testing.T) {
	t.Parallel()
	for _, host := range DefaultDeniedHosts {
		t.Run(host, func(t *testing.T) {
			t.Parallel()
			answers := map[string][]string{host: {publicIPv4}}
			enforce := testPolicy(t, Config{Mode: ModeEnforce}, answers)
			_, err := enforce.ResolveAndValidate(context.Background(), host)
			require.ErrorIs(t, err, ErrAddressBlocked)
			// The host check runs before DNS, so the audit path has to fall through it rather
			// than return early with the resolved addresses missing.
			audit := testPolicy(t, Config{}, answers)
			resolved, err := audit.ResolveAndValidate(context.Background(), host)
			require.NoError(t, err)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(publicIPv4)}, resolved)
		})
	}
	// Matching is exact. A suffix rule would deny names the operator never listed and a prefix
	// one would miss the listed name itself, and neither is something they could see from the
	// ConfigMap.
	policy := testPolicy(t, Config{Mode: ModeEnforce}, map[string][]string{
		"sub.metadata.google.internal": {publicIPv4},
		"xmetadata.google.internal":    {publicIPv4},
	})
	for _, host := range []string{"sub.metadata.google.internal", "xmetadata.google.internal"} {
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.NoError(t, err, "host %q matched a denied entry it only resembles", host)
	}
}

func TestDeniedHostEntriesAreHostGranular(t *testing.T) {
	t.Parallel()
	// A port could never be enforced: the list is matched against a destination host alone, so
	// an entry carrying one would silently deny every port on that host instead of the one the
	// operator wrote. It is dropped, which leaves the host reachable until they fix the entry.
	policy := testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{"registry.internal:5000"}, DeniedIPs: []string{}},
		map[string][]string{"registry.internal": {publicIPv4}})
	_, err := policy.ResolveAndValidate(context.Background(), "registry.internal")
	require.NoError(t, err)
	// Entry and destination are normalized the same way, so case, a trailing dot, or surrounding
	// whitespace on either side cannot evade the list.
	policy = testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{" Registry.Internal. "}, DeniedIPs: []string{}},
		map[string][]string{"registry.internal": {publicIPv4}})
	for _, host := range []string{"registry.internal", "Registry.Internal", "REGISTRY.INTERNAL.", " registry.internal "} {
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.ErrorIs(t, err, ErrAddressBlocked, "host %q", host)
	}
	// An IP literal on the host list is dropped rather than quietly matching a literal
	// destination: addresses belong on the other list, where a CIDR and the IPv6 spellings of
	// the same address work.
	policy = testPolicy(t, Config{Mode: ModeEnforce, DeniedHosts: []string{"10.0.0.1"}, DeniedIPs: []string{}}, nil)
	_, err = policy.ResolveAndValidate(context.Background(), "10.0.0.1")
	require.NoError(t, err)
}

func TestNormalizeHost(t *testing.T) {
	t.Parallel()
	// Every denylist comparison runs on this output, on both sides, so anything it fails to
	// canonicalise is a way to spell a denied destination that does not match its entry.
	for _, tc := range []struct{ input, want string }{
		{input: "registry.example", want: "registry.example"},
		{input: " Registry.Example. ", want: "registry.example"},
		{input: "registry.example:5000", want: "registry.example"},
		{input: "registry-1.sub.example", want: "registry-1.sub.example"},
		{input: "10.0.0.1", want: "10.0.0.1"},
		{input: "10.0.0.1:443", want: "10.0.0.1"},
		{input: "fd01::1", want: "fd01::1"},
		{input: "[fd01::1]", want: "fd01::1"},
		{input: "[fd01::1]:443", want: "fd01::1"},
		{input: "0:0:0:0:0:0:0:1", want: "::1"},
		{input: "::ffff:10.0.0.1", want: "::ffff:10.0.0.1"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			host, err := normalizeHost(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.want, host)
		})
	}
	// A port is validated rather than stripped blindly, and anything that is neither a hostname
	// nor an address is refused outright: letting it through would mean comparing an unparsed
	// authority against the list.
	for _, input := range []string{
		"", " ", "*.internal", "https://registry.internal", "registry.internal/path",
		"user@registry.internal", "registry..internal", "-registry.example", "registry.example-",
		"registry:bad", "registry:0", "registry:65536", "registry:", "fe80::1%eth0",
		"10.0.0.999", "010.0.0.1", "1.2.3.4.5", strings.Repeat("a", 64) + ".example",
		strings.Repeat("a.", 200) + "example",
	} {
		t.Run("invalid/"+input, func(t *testing.T) {
			t.Parallel()
			_, err := normalizeHost(input)
			require.Error(t, err)
		})
	}
}

func TestHasPort(t *testing.T) {
	t.Parallel()
	// A bare IPv6 literal is full of colons but carries no port. Reading one as a port would
	// drop a denied-host entry for a reason the operator cannot see in the ConfigMap -- and an
	// IPv6 entry is dropped anyway, but by the IP-literal check, which says what to do instead.
	for _, tc := range []struct {
		entry string
		want  bool
	}{
		{entry: "registry.example", want: false},
		{entry: "registry.example:5000", want: true},
		{entry: "10.0.0.1", want: false},
		{entry: "10.0.0.1:443", want: true},
		{entry: "fd01::1", want: false},
		{entry: "::1", want: false},
		{entry: "[fd01::1]:443", want: true},
	} {
		t.Run(tc.entry, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, hasPort(tc.entry))
		})
	}
}

func TestIPZonesAreRejected(t *testing.T) {
	t.Parallel()
	// A zone scopes an address to one interface, so the same text names different destinations
	// on different hosts and nothing here can pin it. Every entry point refuses it rather than
	// dropping the zone, which would silently widen whatever it was attached to.
	_, err := normalizeHost("fe80::1%eth0")
	require.Error(t, err)
	_, err = parsePrefix("fe80::1%eth0")
	require.Error(t, err)
	policy := testPolicy(t, Config{Mode: ModeEnforce}, nil)
	_, err = policy.ResolveAndValidate(context.Background(), "fe80::1%eth0")
	require.ErrorContains(t, err, "invalid network host")
	// A resolver that answers with a zoned address is refused too: that address cannot be
	// compared against the list, so passing it through would pin an unchecked destination.
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("fe80::1%eth0")}, nil
	}
	_, err = policy.ResolveAndValidate(context.Background(), "registry.example")
	require.ErrorContains(t, err, "invalid address")
}

func TestNewRejectsOnlyAnInvalidMode(t *testing.T) {
	t.Parallel()
	// The mode is a flag fixed at deploy time, so a typo there is a hard error a controller can
	// fail startup on. The denylists are ConfigMap content that changes under a running process,
	// so a typo there is logged and skipped instead of crashlooping the admission path.
	for _, mode := range []string{"off", "disabled", "warn", "Audit", "ENFORCE", " audit"} {
		_, err := New(Config{Mode: mode})
		require.Error(t, err, "mode %q", mode)
	}
	for _, mode := range []string{"", ModeAudit, ModeEnforce} {
		policy, err := New(Config{Mode: mode})
		require.NoError(t, err, "mode %q", mode)
		require.NotNil(t, policy)
	}
	policy, err := New(Config{})
	require.NoError(t, err)
	assert.Equal(t, ModeAudit, policy.mode, "the unset mode must default to audit, not enforce")
	_, err = New(Config{DeniedHosts: []string{"*.internal"}, DeniedIPs: []string{"10.0.0.0/99"}})
	require.NoError(t, err, "a malformed denylist entry must not fail construction")
}

func TestResolutionFailuresSurfaceWithoutResolverDetails(t *testing.T) {
	t.Parallel()
	// These errors reach admission responses, PolicyReports and Events, so resolved IPs and DNS
	// server addresses stay in the logs. errors.As still has to see through the wrapper, because
	// the transport's proxy deferral keys on *net.DNSError.
	policy := testPolicy(t, Config{}, nil)
	_, err := policy.ResolveAndValidate(context.Background(), "registry.internal")
	require.Error(t, err)
	var dnsError *net.DNSError
	assert.True(t, errors.As(err, &dnsError))
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) {
		return nil, errors.New("DNS server 10.20.30.40 failed")
	}
	_, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "10.20.30.40")
	// An empty answer is not a destination either, and must not be mistaken for "nothing to
	// check" and allowed through.
	policy.lookupNetIP = func(context.Context, string, string) ([]netip.Addr, error) { return nil, nil }
	_, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
	require.ErrorContains(t, err, "no addresses")
}
