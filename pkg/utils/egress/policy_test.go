package egress

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	for _, address := range []string{"10.1.2.3", "172.16.1.1", "192.168.1.1", "100.64.1.1", "fd01::1", "64:ff9b::a01:203"} {
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
			policy = testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{"Registry.Internal.:5000"}}, answers)
			resolved, err = policy.ResolveAndValidate(context.Background(), "registry.internal")
			require.NoError(t, err)
			assert.Equal(t, []netip.Addr{netip.MustParseAddr(address)}, resolved, "an exempt hostname must still be resolved and pinned")
		})
	}
}

func TestAllowlistValidation(t *testing.T) {
	t.Parallel()
	for _, entry := range []string{"", " ", "*.internal", "https://registry.internal", "registry.internal/path", "user@registry.internal", "10.0.0.0/99", "10.0.0.999", "010.0.0.1", "registry..internal", "registry:bad", "registry:65536", "registry:", "fe80::1%eth0", "-registry.example"} {
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
	policy := testPolicy(t, Config{Mode: ModeEnforce, Allowlist: []string{"10.0.0.0/16", "[fd01::1]:443", "192.168.1.1"}}, map[string][]string{
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
		policy := testPolicy(t, Config{Allowlist: []string{host}}, map[string][]string{host: {"93.184.216.34"}})
		_, err := policy.ResolveAndValidate(context.Background(), host)
		require.ErrorIs(t, err, ErrAddressBlocked)
	}
}
