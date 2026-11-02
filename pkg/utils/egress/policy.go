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
		if hasPort(entry) {
			return nil, fmt.Errorf("invalid egress allowlist entry %q: ports are not supported, use the bare host or IP address", entry)
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

// hasPort reports whether an allowlist entry carries a port. Exemptions are matched against
// a destination host alone -- resolveAndValidate only ever receives a hostname -- so a port
// written here could never be enforced and would silently exempt every port on that host.
// Reject it at startup instead. Bare IPv6 literals are not ports despite their colons. Valid
// CIDRs never reach this -- New parses them first and skips ahead -- and malformed ones fall
// through to normalizeHost, which reports them.
func hasPort(entry string) bool {
	if _, err := netip.ParseAddr(entry); err == nil {
		return false
	}
	_, _, err := net.SplitHostPort(entry)
	return err == nil
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
	// RFC 2544 benchmarking space. net.IP reports it as global unicast and not private, so
	// without this it would be reachable in enforce mode with no allowlist entry despite
	// never being globally routable.
	benchmarkNetwork = netip.MustParsePrefix("198.18.0.0/15")
	// More special-purpose space Go reports as global unicast and not private. Each one can
	// address local infrastructure or steer routing, so each is private-class: audit logs it
	// and enforce needs an allowlist entry. The documentation ranges belong here for the same
	// reason 240.0.0.0/4 does -- "never globally routable" is not "never routed". Nothing stops
	// a cluster from pointing 203.0.113.0/24 at an internal service, and the address checked
	// here is exactly the one dialed, so in enforce mode a registry fetch would otherwise reach
	// it. An operator who genuinely runs a registry in any of these ranges allowlists it.
	//
	// This list was grown one range at a time across four review rounds, which is how a gap
	// like fec0::/10 survived. It is now derived from the IANA special-purpose registries
	// instead: every entry marked "Globally Reachable: False" that Go does not already report
	// as loopback, link-local, unspecified, multicast or private is listed here or hard-blocked
	// above. Entries marked True are deliberately absent -- 2620:4f:8000::/48, 192.31.196.0/24,
	// 192.175.48.0/24 (AS112) and 192.52.193.0/24 (AMT) are real routed services. Re-derive from
	// those registries when adding to this list; do not append a single range in isolation.
	protocolAssignments   = netip.MustParsePrefix("192.0.0.0/24")    // IETF assignments: 192.0.0.1 is the DS-Lite AFTR the local CPE answers on.
	sixToFourRelayAnycast = netip.MustParsePrefix("192.88.99.0/24")  // Deprecated 6to4 relay anycast: reaches whichever relay the local network advertises.
	reservedNetwork       = netip.MustParsePrefix("240.0.0.0/4")     // Reserved for future use: only a local stack or appliance can answer here.
	protocolAssignmentsV6 = netip.MustParsePrefix("2001::/23")       // IPv6 IETF assignments, including Teredo 2001::/32 tunnel endpoints.
	discardOnlyV6         = netip.MustParsePrefix("100::/64")        // Discard-only: a route here is local policy, not a destination.
	testNet1              = netip.MustParsePrefix("192.0.2.0/24")    // RFC 5737 TEST-NET-1: no public route exists, so a reply came from something the local network put there.
	testNet2              = netip.MustParsePrefix("198.51.100.0/24") // RFC 5737 TEST-NET-2: likewise, only a local route can carry a fetch here.
	testNet3              = netip.MustParsePrefix("203.0.113.0/24")  // RFC 5737 TEST-NET-3: likewise.
	documentationV6       = netip.MustParsePrefix("2001:db8::/32")   // RFC 3849 IPv6 documentation space: disjoint from 2001::/23, and routed only where an operator chose to.
	siteLocalV6           = netip.MustParsePrefix("fec0::/10")       // Deprecated IPv6 site-local (RFC 3879). Adjacent to fe80::/10, not covered by it, and Go reports it as ordinary global unicast; stacks and appliances still answer here.
	documentationV6Alt    = netip.MustParsePrefix("3fff::/20")       // RFC 9637 IPv6 documentation space, the 2024 addition alongside 2001:db8::/32.
	segmentRoutingV6      = netip.MustParsePrefix("5f00::/16")       // RFC 9602 SRv6 SIDs: these name segments inside a routing domain, so a reply means local routing infrastructure.
	localUseNAT64V6       = netip.MustParsePrefix("64:ff9b:1::/48")  // RFC 8215 local-use NAT64. Disjoint from the well-known 64:ff9b::/96 that EmbeddedIPv4 unwraps, and the translator it names is local by definition.
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

// privateAddress is only ever called on a candidate from addressCandidates, which has
// already unmapped and extracted any embedded IPv4, so ::ffff:192.0.0.1, 64:ff9b::c000:1,
// and 2002:c000:1:: cannot spell their way past an IPv4 prefix listed here. The IPv6-only
// prefixes need no equivalent: the mapped and embedded forms carry an IPv4 address and so
// cannot spell one of them, and addressCandidates always keeps the original address too.
func privateAddress(address netip.Addr) bool {
	return address.IsPrivate() || carrierGradeNAT.Contains(address) || benchmarkNetwork.Contains(address) ||
		protocolAssignments.Contains(address) || sixToFourRelayAnycast.Contains(address) ||
		reservedNetwork.Contains(address) || protocolAssignmentsV6.Contains(address) ||
		discardOnlyV6.Contains(address) || testNet1.Contains(address) || testNet2.Contains(address) ||
		testNet3.Contains(address) || documentationV6.Contains(address) || siteLocalV6.Contains(address) ||
		documentationV6Alt.Contains(address) || segmentRoutingV6.Contains(address) ||
		localUseNAT64V6.Contains(address)
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
