// Package egress checks the destination of every image registry request against two
// operator-owned denylists.
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

// Config is the operator's denylist: a destination is denied when it matches one of these lists
// and permitted otherwise. There is no allowlist and no built-in notion of a private address.
//
// DeniedHosts entries are exact hostnames; DeniedIPs entries are CIDRs or bare IPs. Both come
// from the kyverno ConfigMap, which the Helm chart populates with the recommended defaults; there
// is no built-in copy, so an empty or missing list denies nothing.
type Config struct {
	Mode        string
	DeniedHosts []string
	DeniedIPs   []string
}

// Policy is immutable after construction and can be shared by transports.
type Policy struct {
	mode        string
	hosts       map[string]struct{}
	prefixes    []netip.Prefix
	lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
}

// New defaults to audit mode, which logs what enforce would reject and connects anyway.
//
// A malformed denylist entry is logged and skipped rather than failing startup: these come from
// a ConfigMap edited at runtime, and a typo there must not crashloop admission. Only the mode is
// a hard error, being a flag fixed at deploy time.
func New(config Config) (*Policy, error) {
	mode := config.Mode
	if mode == "" {
		mode = ModeAudit
	}
	if mode != ModeAudit && mode != ModeEnforce {
		return nil, fmt.Errorf("invalid egress mode %q: expected audit or enforce", mode)
	}
	logger := logging.WithName("egress")
	policy := &Policy{mode: mode, hosts: make(map[string]struct{}), lookupNetIP: net.DefaultResolver.LookupNetIP}
	for _, entry := range config.DeniedHosts {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// Matching runs on the host alone, so an entry with a port would silently deny all ports.
		if hasPort(entry) {
			logger.Error(nil, "ignoring denied host: ports are not supported, use the bare host", "entry", entry)
			continue
		}
		host, err := normalizeHost(entry)
		if err != nil {
			logger.Error(err, "ignoring invalid denied host", "entry", entry)
			continue
		}
		// Hosts are compared by name, so an IP here would never match.
		if _, err := netip.ParseAddr(host); err == nil {
			logger.Error(nil, "ignoring denied host: this is an IP address, list it under denied IPs", "entry", entry)
			continue
		}
		policy.hosts[host] = struct{}{}
	}
	for _, entry := range config.DeniedIPs {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		prefix, err := parsePrefix(entry)
		if err != nil {
			logger.Error(err, "ignoring invalid denied IP", "entry", entry)
			continue
		}
		policy.prefixes = append(policy.prefixes, prefix)
	}
	return policy, nil
}

// parsePrefix accepts a CIDR or bare IP. An IPv4-mapped prefix is rewritten to IPv4 form since
// matching runs on unmapped candidates; one shorter than /96 would span both families.
func parsePrefix(entry string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(entry); err == nil {
		if prefix.Addr().Zone() != "" {
			return netip.Prefix{}, errors.New("IP zones are not supported")
		}
		if prefix.Addr().Is4In6() {
			if prefix.Bits() < 96 {
				return netip.Prefix{}, errors.New("an IPv4-mapped prefix shorter than /96 spans both address families")
			}
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		return prefix.Masked(), nil
	}
	address, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, errors.New("expected a CIDR or an IP address")
	}
	if address.Zone() != "" {
		return netip.Prefix{}, errors.New("IP zones are not supported")
	}
	address = address.Unmap()
	return netip.PrefixFrom(address, address.BitLen()), nil
}

// hasPort reports whether a denied-host entry carries a port. Bare IPv6 literals are not ports
// despite their colons.
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

// ResolveAndValidate runs steps 1-3 of the policy: deny a listed host, resolve, deny a listed
// address. Enforce rejects, audit logs. Step 4, sending the request, is the caller's.
func (p *Policy) ResolveAndValidate(ctx context.Context, host string) ([]netip.Addr, error) {
	host, err := normalizeHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid network host: %w", err)
	}
	if _, denied := p.hosts[host]; denied {
		logging.WithName("egress").V(2).Info("denied network host", "host", host, "mode", p.mode)
		if p.mode == ModeEnforce {
			return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
		}
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
	for _, address := range addresses {
		if err := p.checkAddress(host, address); err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

// checkAddress is step 3 for one address. The transport also runs it on the address each
// connection actually reached, since that DNS answer can differ from the one checked above.
func (p *Policy) checkAddress(host string, address netip.Addr) error {
	if !address.IsValid() || address.Zone() != "" {
		return fmt.Errorf("network host %q resolved to an invalid address", host)
	}
	for _, candidate := range addressCandidates(address) {
		if !p.deniedAddress(candidate) {
			continue
		}
		logging.WithName("egress").V(2).Info("denied network destination", "host", host, "ip", candidate.String(), "mode", p.mode)
		if p.mode == ModeEnforce {
			return &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
		}
	}
	return nil
}

func (p *Policy) deniedAddress(address netip.Addr) bool {
	for _, prefix := range p.prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// addressCandidates also checks the embedded IPv4 address: a CIDR cannot express that the
// mapped, NAT64 and 6to4 spellings all reach the same IPv4 address, so without this any denied
// IPv4 entry is bypassable by writing it as IPv6. The original is kept for IPv6-only entries.
func addressCandidates(address netip.Addr) []netip.Addr {
	candidates := []netip.Addr{address.Unmap()}
	if embedded := EmbeddedIPv4(net.IP(address.AsSlice())); embedded != nil {
		if parsed, ok := netip.AddrFromSlice(embedded); ok {
			candidates = append(candidates, parsed.Unmap())
		}
	}
	return candidates
}

// EmbeddedIPv4 extracts the IPv4 address carried by an IPv6 address in a form net.IP.To4 does
// not recognise: IPv4-compatible (::a.b.c.d), NAT64 (64:ff9b::a.b.c.d), and 6to4
// (2002:aabb:ccdd::). Each is a distinct spelling of an address a denied IPv4 CIDR is meant to
// cover: without this, 64:ff9b::a9fe:a9fe reaches metadata even though 169.254.0.0/16 is denied.
//
// The IPv4-compatible branch also matches low IPv6 addresses never meant as an IPv4 encoding
// (::1 yields 0.0.0.1). That only widens the denylist into reserved space, so it is left as is.
func EmbeddedIPv4(ip net.IP) net.IP {
	if ip.To4() != nil || len(ip.To16()) != net.IPv6len {
		return nil
	}
	ip = ip.To16()
	isZero := func(b []byte) bool {
		for _, c := range b {
			if c != 0 {
				return false
			}
		}
		return true
	}
	// ::a.b.c.d — IPv4-compatible IPv6 (RFC 4291).
	if isZero(ip[:12]) {
		return net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}
	// 64:ff9b::a.b.c.d — NAT64 well-known prefix (RFC 6052).
	if ip[0] == 0x00 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b && isZero(ip[4:12]) {
		return net.IPv4(ip[12], ip[13], ip[14], ip[15])
	}
	// 2002:aabb:ccdd::/48 — 6to4 (RFC 3056), which carries the IPv4 address in bytes 2-5
	// rather than at the end. 2002:a9fe:a9fe:: is 169.254.169.254.
	if ip[0] == 0x20 && ip[1] == 0x02 {
		return net.IPv4(ip[2], ip[3], ip[4], ip[5])
	}
	return nil
}

// hostError keeps resolved IPs and DNS server addresses out of admission errors, PolicyReports
// and Events, while preserving errors.Is/errors.As behavior.
type hostError struct {
	operation, host string
	err             error
}

func (e *hostError) Error() string { return fmt.Sprintf("%s network host %q", e.operation, e.host) }
func (e *hostError) Unwrap() error { return e.err }
