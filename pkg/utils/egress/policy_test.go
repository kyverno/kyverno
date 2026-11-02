package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures standing in for an ordinary public registry or proxy address, shared by every test
// in this package. The RFC 5737/3849 documentation ranges (192.0.2.0/24, 198.51.100.0/24,
// 203.0.113.0/24, 2001:db8::/32) cannot play that role: policy.go classifies them as
// private-class, so a fixture written there is logged as private in audit mode and rejected
// outright in enforce mode. Use documentation addresses only where the point of the test is
// that they are rejected. These are real public addresses, but nothing is ever contacted --
// every resolver and dialer in these tests is stubbed.
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

func TestHardBlocksCannotBeExempted(t *testing.T) {
	t.Parallel()
	for _, address := range []string{
		"127.0.0.1", "::1", "169.254.169.254", "fe80::1", "0.0.0.0", "::", "224.0.0.1", "ff02::1", "255.255.255.255",
		"::ffff:169.254.169.254", "::169.254.169.254", "64:ff9b::a9fe:a9fe", "2002:a9fe:a9fe::",
		"100.100.100.200", "168.63.129.16", "fd00:ec2::254",
	} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			for _, mode := range []string{ModeAudit, ModeEnforce} {
				policy := testPolicy(t, Config{Mode: mode, Allowlist: []string{"exempt.example", "0.0.0.0/0", "::/0"}}, map[string][]string{"exempt.example": {address}})
				_, err := policy.ResolveAndValidate(context.Background(), "exempt.example")
				require.ErrorIs(t, err, ErrAddressBlocked)
				assert.NotContains(t, err.Error(), address)
			}
		})
	}
}

func TestPrivateNetworkCompatibilityAndExemptions(t *testing.T) {
	t.Parallel()
	// The special-purpose ranges below are reported as global unicast and not private by Go,
	// so each one is here to prove it is still private-class: reachable in audit mode, and in
	// enforce mode only with an exemption. The last two are alternate spellings of 192.0.0.1.
	for _, address := range []string{
		"10.1.2.3", "172.16.1.1", "192.168.1.1", "100.64.1.1", "fd01::1", "64:ff9b::a01:203",
		"192.0.0.1", "192.88.99.1", "240.0.0.1", "2001:0:4136:e378:8000:63bf:3fff:fdd2", "100::1",
		"192.0.2.1", "198.51.100.1", "203.0.113.1", "2001:db8::1",
		"fec0::1", "feff:ffff::1", "3fff::1", "5f00::1", "64:ff9b:1::1",
		"::ffff:192.0.0.1", "64:ff9b::c000:1",
	} {
		t.Run(address, func(t *testing.T) {
			t.Parallel()
			answers := map[string][]string{"registry.internal": {address}}
			policy := testPolicy(t, Config{}, answers)
			resolved, err := policy.ResolveAndValidate(context.Background(), "Registry.Internal.")
			require.NoError(t, err)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(address)}, resolved)
			policy = testPolicy(t, Config{Mode: ModeEnforce}, answers)
			_, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.ErrorIs(t, err, ErrAddressBlocked)
			policy = testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{"Registry.Internal."}}, answers)
			resolved, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.NoError(t, err)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(address)}, resolved, "an exempt hostname must still be resolved and pinned")
		})
	}
}

// Special-purpose space is private-class rather than hard-blocked, so an operator who
// genuinely runs a registry there can still reach it with an address or CIDR exemption.
func TestSpecialPurposeRangesAreExemptableByAddress(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, address, exemption string
	}{
		{name: "IETF protocol assignments", address: "192.0.0.1", exemption: "192.0.0.1"},
		{name: "6to4 relay anycast", address: "192.88.99.1", exemption: "192.88.99.0/24"},
		{name: "reserved for future use", address: "240.0.0.1", exemption: "240.0.0.0/4"},
		{name: "teredo", address: "2001:0:4136:e378:8000:63bf:3fff:fdd2", exemption: "2001::/23"},
		{name: "IPv6 discard only", address: "100::1", exemption: "100::/64"},
		// Documentation space is unroutable in public, not unroutable everywhere: a cluster is
		// free to point it at an internal service, so it is private-class too.
		{name: "documentation TEST-NET-1", address: "192.0.2.1", exemption: "192.0.2.0/24"},
		{name: "documentation TEST-NET-2", address: "198.51.100.1", exemption: "198.51.100.0/24"},
		{name: "documentation TEST-NET-3", address: "203.0.113.1", exemption: "203.0.113.0/24"},
		{name: "IPv6 documentation", address: "2001:db8::1", exemption: "2001:db8::/32"},
		// Go reports every one of these as ordinary global unicast with IsPrivate false, so
		// each needs its own prefix or enforce mode lets a registry reference reach it.
		{name: "deprecated IPv6 site local", address: "fec0::1", exemption: "fec0::/10"},
		// fec0::/10 is adjacent to fe80::/10, not inside it, so the hard block on link-local
		// does not reach the top of the range either.
		{name: "deprecated IPv6 site local upper bound", address: "feff:ffff::1", exemption: "fec0::/10"},
		{name: "IPv6 documentation RFC 9637", address: "3fff::1", exemption: "3fff::/20"},
		{name: "SRv6 SID", address: "5f00::1", exemption: "5f00::/16"},
		{name: "local use NAT64", address: "64:ff9b:1::1", exemption: "64:ff9b:1::/48"},
		{name: "IPv4 mapped protocol assignment", address: "::ffff:192.0.0.1", exemption: "192.0.0.0/24"},
		{name: "NAT64 protocol assignment", address: "64:ff9b::c000:1", exemption: "192.0.0.0/24"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			answers := map[string][]string{"registry.internal": {tc.address}}
			policy := testPolicy(t, Config{Mode: ModeEnforce}, answers)
			_, err := policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.ErrorIs(t, err, ErrAddressBlocked)
			policy = testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{tc.exemption}}, answers)
			resolved, err := policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.NoError(t, err)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(tc.address)}, resolved)
		})
	}
}

// The private-class list is derived from the IANA special-purpose registries by taking every
// "Globally Reachable: False" entry. The entries marked True are real routed services and must
// stay reachable in enforce mode with no exemption, or this guard breaks ordinary fetches while
// claiming to block only local infrastructure. This is the other half of the derivation rule in
// policy.go: that list says what to include, and this test pins what deliberately stays out.
func TestGloballyReachableSpecialPurposeRangesStayPublic(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, address string }{
		{name: "AS112 v4", address: "192.31.196.1"},
		{name: "AS112 v4 direct delegation", address: "192.175.48.1"},
		{name: "AMT", address: "192.52.193.1"},
		{name: "AS112 v6 direct delegation", address: "2620:4f:8000::1"},
		// 6to4 carrying a public IPv4 is a public destination. 2002::/16 as a whole is not
		// private-class; EmbeddedIPv4 unwraps it and the embedded address decides.
		{name: "6to4 wrapping a public address", address: "2002:5db8:d822::1"},
		// fec0::/10 sits directly above fe80::/10 and fc00::/7 directly below it. fe00::1 falls
		// between ULA and link-local, so it catches any of the three over-reaching downward.
		// The upper edge is pinned by the feff:ffff::1 case in the exemptable-ranges test.
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
			answers := map[string][]string{"registry.example": {tc.address}}
			policy := testPolicy(t, Config{Mode: ModeEnforce}, answers)
			resolved, err := policy.ResolveAndValidate(context.Background(), "registry.example")
			require.NoError(t, err, "enforce mode must not need an exemption for a globally routed address")
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(tc.address)}, resolved)
		})
	}
}

func TestAllowlistValidation(t *testing.T) {
	t.Parallel()
	// Port-bearing entries are rejected rather than silently exempting every port on the
	// host: exemptions are matched against the destination host alone, so a port written
	// here could never be enforced.
	for _, entry := range []string{"", " ", "*.internal", "https://registry.internal", "registry.internal/path", "user@registry.internal", "10.0.0.0/99", "10.0.0.999", "010.0.0.1", "registry..internal", "registry:bad", "registry:65536", "registry:", "fe80::1%eth0", "-registry.example", "registry.internal:5000", "[fd01::1]:443", "10.0.0.1:443"} {
		t.Run(entry, func(t *testing.T) {
			t.Parallel()
			_, err := New(Config{Allowlist: []string{entry}})
			require.Error(t, err)
		})
	}
	_, err := New(Config{Mode: "off"})
	require.Error(t, err)
}

func TestCIDRAndIPExemptions(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{"10.0.0.0/16", "fd01::1", "192.168.1.1"}}, map[string][]string{
		"allowed.example":  {"10.0.1.2", "fd01::1", "192.168.1.1"},
		"mixed.example":    {"10.0.1.2", "10.1.1.1"},
		"embedded.example": {"64:ff9b::a00:102"},
	})
	_, err := policy.ResolveAndValidate(context.Background(), "allowed.example")
	require.NoError(t, err)
	_, err = policy.ResolveAndValidate(context.Background(), "embedded.example")
	require.NoError(t, err)
	_, err = policy.ResolveAndValidate(context.Background(), "mixed.example")
	require.ErrorIs(t, err, ErrAddressBlocked)
}

func TestResolutionFailuresAreNotExempt(t *testing.T) {
	t.Parallel()
	policy := testPolicy(t, Config{Allowlist: []string{"registry.internal"}}, nil)
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
}

func TestMetadataHostnamesCannotBeExempted(t *testing.T) {
	t.Parallel()
	for _, host := range []string{"metadata.internal", "metadata.google.internal", "instance-data.ec2.internal"} {
		policy := testPolicy(t, Config{Allowlist: []string{host}}, map[string][]string{host: {publicIPv4}})
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.ErrorIs(t, err, ErrAddressBlocked)
	}
}

func TestDialAddressesFallsBackWithinSameFamily(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		budget     time.Duration
		candidates int
	}{
		{name: "ordinary deadline", budget: 3 * time.Second, candidates: 2},
		{name: "short deadline", budget: 500 * time.Millisecond, candidates: 2},
		{name: "many candidates", budget: 3 * time.Second, candidates: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), tc.budget)
			defer cancel()
			peer, expected := net.Pipe()
			defer peer.Close()
			defer expected.Close()
			addresses := make([]net.IP, 0, tc.candidates)
			for i := range tc.candidates {
				addresses = append(addresses, net.ParseIP(fmt.Sprintf(publicIPv4Nth, i+1)))
			}
			var wantAttempts []string
			for _, ip := range addresses {
				wantAttempts = append(wantAttempts, net.JoinHostPort(ip.String(), "443"))
			}
			var attempted []string
			var deadlines []time.Time
			connection, err := DialAddresses(ctx, func(attemptCtx context.Context, _, address string) (net.Conn, error) {
				attempted = append(attempted, address)
				deadline, ok := attemptCtx.Deadline()
				require.True(t, ok)
				deadlines = append(deadlines, deadline)
				if len(attempted) < tc.candidates {
					// Model blackholed addresses before the usable final candidate.
					// Each attempt must leave time for every remaining address.
					<-attemptCtx.Done()
					return nil, attemptCtx.Err()
				}
				return expected, nil
			}, "tcp", "443", addresses)
			require.NoError(t, err)
			assert.Same(t, expected, connection)
			assert.NoError(t, ctx.Err(), "fallback must finish inside the original aggregate deadline")
			assert.Equal(t, wantAttempts, attempted)
			require.Len(t, deadlines, tc.candidates)
			for i := 1; i < len(deadlines); i++ {
				assert.True(t, deadlines[i-1].Before(deadlines[i]))
			}
			outerDeadline, _ := ctx.Deadline()
			assert.Equal(t, outerDeadline, deadlines[len(deadlines)-1])
		})
	}
}

func TestDialAddressesCancellationStopsRemainingCandidates(t *testing.T) {
	t.Parallel()
	for _, alreadyCanceled := range []bool{true, false} {
		name := "during first attempt"
		if alreadyCanceled {
			name = "before dialing"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if alreadyCanceled {
				cancel()
			}
			attempts := 0
			connection, err := DialAddresses(ctx, func(attemptCtx context.Context, _, _ string) (net.Conn, error) {
				attempts++
				cancel()
				<-attemptCtx.Done()
				return nil, attemptCtx.Err()
			}, "tcp", "443", []net.IP{net.ParseIP(publicIPv4), net.ParseIP(publicIPv4Alt)})
			require.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, connection)
			if alreadyCanceled {
				assert.Zero(t, attempts)
			} else {
				assert.Equal(t, 1, attempts)
			}
		})
	}
}

func TestDialAddressesWithoutDeadline(t *testing.T) {
	t.Parallel()
	peer, expected := net.Pipe()
	defer peer.Close()
	defer expected.Close()
	attempts := 0
	connection, err := DialAddresses(context.Background(), func(ctx context.Context, _, _ string) (net.Conn, error) {
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline, "the helper must not invent a caller timeout")
		attempts++
		if attempts == 1 {
			return nil, errors.New("connection refused")
		}
		return expected, nil
	}, "tcp", "443", []net.IP{net.ParseIP(publicIPv4), net.ParseIP(publicIPv4Alt)})
	require.NoError(t, err)
	assert.Same(t, expected, connection)
	assert.Equal(t, 2, attempts)
}

func TestPartialDialDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for _, tc := range []struct {
		name      string
		budget    time.Duration
		remaining int
		want      time.Duration
	}{
		{name: "registry budget split", budget: 5 * time.Second, remaining: 2, want: 2500 * time.Millisecond},
		{name: "apiCall budget split", budget: 30 * time.Second, remaining: 4, want: 7500 * time.Millisecond},
		{name: "many candidates share budget", budget: 5 * time.Second, remaining: 8, want: 625 * time.Millisecond},
		{name: "two seconds available to every candidate", budget: 16 * time.Second, remaining: 8, want: 2 * time.Second},
		{name: "short caller budget", budget: 500 * time.Millisecond, remaining: 2, want: 250 * time.Millisecond},
		{name: "last candidate", budget: 3 * time.Second, remaining: 1, want: 3 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deadline, err := partialDialDeadline(now, now.Add(tc.budget), tc.remaining)
			require.NoError(t, err)
			assert.Equal(t, now.Add(tc.want), deadline)
		})
	}
	_, err := partialDialDeadline(now, now, 2)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	deadline, err := partialDialDeadline(now, time.Time{}, 2)
	require.NoError(t, err)
	assert.True(t, deadline.IsZero())
}
