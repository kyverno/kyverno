// Package egress validates and pins network destinations shared by registry and
// signature verification clients.
package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	"github.com/kyverno/kyverno/pkg/logging"
)

const (
	ModeAudit   = "audit"
	ModeEnforce = "enforce"
)

// ErrAddressBlocked identifies a destination rejected by the egress policy.
var ErrAddressBlocked = errors.New("network destination blocked by egress policy")

// Config controls private network access. Hard-blocked addresses are never exempt.
type Config struct {
	Allowlist []string
	Mode      string
}

// Policy is immutable after construction and can be shared by transports.
type Policy struct {
	mode        string
	hosts       map[string]struct{}
	prefixes    []netip.Prefix
	lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
}

// New validates every exemption and defaults to audit mode for compatibility with
// private registries and enterprise signature verification services.
func New(config Config) (*Policy, error) {
	mode := config.Mode
	if mode == "" {
		mode = ModeAudit
	}
	if mode != ModeAudit && mode != ModeEnforce {
		return nil, fmt.Errorf("invalid egress mode %q: expected audit or enforce", mode)
	}
	policy := &Policy{mode: mode, hosts: make(map[string]struct{}), lookupNetIP: net.DefaultResolver.LookupNetIP}
	for _, entry := range config.Allowlist {
		entry = strings.TrimSpace(entry)
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			if prefix.Addr().Is4In6() {
				if prefix.Bits() < 96 {
					return nil, fmt.Errorf("invalid egress allowlist CIDR %q", entry)
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			policy.prefixes = append(policy.prefixes, prefix.Masked())
			continue
		}
		host, err := normalizeHost(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid egress allowlist entry %q: %w", entry, err)
		}
		if address, err := netip.ParseAddr(host); err == nil {
			address = address.Unmap()
			policy.prefixes = append(policy.prefixes, netip.PrefixFrom(address, address.BitLen()))
		} else {
			policy.hosts[host] = struct{}{}
		}
	}
	return policy, nil
}

func normalizeHost(authority string) (string, error) {
	authority = strings.TrimSpace(authority)
	if authority == "" {
		return "", errors.New("host is empty")
	}
	if parsed, err := netip.ParseAddr(authority); err == nil {
		if parsed.Zone() != "" {
			return "", errors.New("IP zones are not supported")
		}
		return parsed.String(), nil
	}
	if host, port, err := net.SplitHostPort(authority); err == nil {
		if port == "" {
			return "", errors.New("port is empty")
		}
		for _, digit := range port {
			if digit < '0' || digit > '9' {
				return "", errors.New("port must be numeric")
			}
		}
		if portNumber, err := strconv.Atoi(port); err != nil || portNumber < 1 || portNumber > 65535 {
			return "", errors.New("port is out of range")
		}
		authority = host
	} else if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		authority = authority[1 : len(authority)-1]
	}
	if parsed, err := netip.ParseAddr(authority); err == nil {
		if parsed.Zone() != "" {
			return "", errors.New("IP zones are not supported")
		}
		return parsed.String(), nil
	}
	host := strings.ToLower(strings.TrimSuffix(authority, "."))
	if strings.Contains(host, ".") && strings.Trim(host, "0123456789.") == "" {
		return "", errors.New("invalid IPv4 address")
	}
	if len(host) == 0 || len(host) > 253 {
		return "", errors.New("invalid hostname length")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errors.New("invalid hostname label")
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
				return "", errors.New("expected a hostname, IP address, or CIDR")
			}
		}
	}
	return host, nil
}

// ResolveAndValidate always resolves and validates every answer, including for an
// exempt hostname. Its returned addresses must be used for dialing to prevent
// a second DNS lookup from changing the destination.
func (p *Policy) ResolveAndValidate(ctx context.Context, host string) ([]netip.Addr, error) {
	return p.resolveAndValidate(ctx, host, false)
}

func (p *Policy) resolveAndValidate(ctx context.Context, host string, proxy bool) ([]netip.Addr, error) {
	host, err := normalizeHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid network host: %w", err)
	}
	if host == "metadata.internal" || host == "metadata.google.internal" || host == "instance-data.ec2.internal" {
		return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
	}
	var addresses []netip.Addr
	if address, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{address}
	} else {
		addresses, err = p.lookupNetIP(ctx, "ip", host)
		if err != nil {
			logging.WithName("egress").V(2).Info("failed to resolve network destination", "host", host, "error", err.Error())
			return nil, &hostError{operation: "failed to resolve", host: host, err: err}
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("network host %q resolved to no addresses", host)
	}
	_, hostAllowed := p.hosts[host]
	for _, address := range addresses {
		if !address.IsValid() || address.Zone() != "" {
			return nil, fmt.Errorf("network host %q resolved to an invalid address", host)
		}
		for _, candidate := range addressCandidates(address) {
			if hardBlocked(candidate) {
				logging.WithName("egress").V(2).Info("blocked network destination", "host", host, "ip", candidate.String())
				return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
			}
			if !privateAddress(candidate) || proxy || hostAllowed || p.allowedAddress(address, candidate) {
				continue
			}
			logging.WithName("egress").V(2).Info("private network destination requires an exemption in enforce mode", "host", host, "ip", candidate.String(), "mode", p.mode)
			if p.mode == ModeEnforce {
				return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
			}
		}
	}
	return addresses, nil
}

func (p *Policy) allowedAddress(address, embedded netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(address.Unmap()) || prefix.Contains(embedded) {
			return true
		}
	}
	return false
}

var (
	carrierGradeNAT = netip.MustParsePrefix("100.64.0.0/10")
	zeroNetwork     = netip.MustParsePrefix("0.0.0.0/8")
	// Some cloud metadata services use otherwise private or public addresses.
	metadataAddresses = map[netip.Addr]struct{}{
		netip.MustParseAddr("100.100.100.200"): {},
		netip.MustParseAddr("168.63.129.16"):   {},
		netip.MustParseAddr("fd00:ec2::254"):   {},
	}
)

func hardBlocked(address netip.Addr) bool {
	_, metadata := metadataAddresses[address]
	return !address.IsValid() || !address.IsGlobalUnicast() || address.IsLoopback() || address.IsLinkLocalUnicast() || zeroNetwork.Contains(address) || metadata
}

func privateAddress(address netip.Addr) bool {
	return address.IsPrivate() || carrierGradeNAT.Contains(address)
}

func addressCandidates(address netip.Addr) []netip.Addr {
	candidates := []netip.Addr{address.Unmap()}
	if embedded := EmbeddedIPv4(net.IP(address.AsSlice())); embedded != nil {
		if parsed, ok := netip.AddrFromSlice(embedded); ok {
			candidates = append(candidates, parsed.Unmap())
		}
	}
	return candidates
}

// hostError keeps resolved IPs and DNS server addresses out of admission errors,
// PolicyReports, and Events while preserving errors.Is/errors.As behavior.
type hostError struct {
	operation, host string
	err             error
}

func (e *hostError) Error() string { return fmt.Sprintf("%s network host %q", e.operation, e.host) }
func (e *hostError) Unwrap() error { return e.err }
