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

// Config is the operator's denylist: a destination is denied when it matches one of these lists
// and permitted otherwise. There is no allowlist and no built-in notion of a private address.
//
// DeniedHosts entries are exact hostnames; DeniedIPs entries are CIDRs or bare IPs. nil means
// unset and falls back to the shipped defaults; an empty non-nil slice is an operator clearing
// the list and is honored.
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
	for _, entry := range listOrDefault(config.DeniedHosts, DefaultDeniedHosts) {
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
	for _, entry := range listOrDefault(config.DeniedIPs, DefaultDeniedIPs) {
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

// listOrDefault keeps nil (never configured) distinct from empty (cleared on purpose).
func listOrDefault(configured, shipped []string) []string {
	if configured == nil {
		return shipped
	}
	return configured
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

// ResolveAndValidate is the whole policy: deny a listed host, resolve, deny a listed address,
// otherwise hand the request on. Enforce rejects, audit logs. It returns the addresses the
// caller must dial, so a second DNS lookup cannot move the destination after the check.
func (p *Policy) ResolveAndValidate(ctx context.Context, host string) ([]netip.Addr, error) {
	return p.resolveAndValidate(ctx, host, false)
}

func (p *Policy) resolveAndValidate(ctx context.Context, host string, proxy bool) ([]netip.Addr, error) {
	host, err := normalizeHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid network host: %w", err)
	}
	logger := logging.WithName("egress")
	// A proxy endpoint comes from the environment, not from an attacker-chosen image reference,
	// so the denylist does not apply to it -- otherwise the shipped loopback and RFC 1918
	// defaults would break every in-cluster egress proxy. The target behind it is still checked.
	if !proxy {
		if _, denied := p.hosts[host]; denied {
			logger.V(2).Info("denied network host", "host", host, "mode", p.mode)
			if p.mode == ModeEnforce {
				return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
			}
		}
	}
	var addresses []netip.Addr
	if address, err := netip.ParseAddr(host); err == nil {
		addresses = []netip.Addr{address}
	} else {
		addresses, err = p.lookupNetIP(ctx, "ip", host)
		if err != nil {
			logger.V(2).Info("failed to resolve network destination", "host", host, "error", err.Error())
			return nil, &hostError{operation: "failed to resolve", host: host, err: err}
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("network host %q resolved to no addresses", host)
	}
	for _, address := range addresses {
		if !address.IsValid() || address.Zone() != "" {
			return nil, fmt.Errorf("network host %q resolved to an invalid address", host)
		}
		if proxy {
			continue
		}
		for _, candidate := range addressCandidates(address) {
			if !p.deniedAddress(candidate) {
				continue
			}
			logger.V(2).Info("denied network destination", "host", host, "ip", candidate.String(), "mode", p.mode)
			if p.mode == ModeEnforce {
				return nil, &hostError{operation: "connection blocked to", host: host, err: ErrAddressBlocked}
			}
		}
	}
	return addresses, nil
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

// hostError keeps resolved IPs and DNS server addresses out of admission errors, PolicyReports
// and Events, while preserving errors.Is/errors.As behavior.
type hostError struct {
	operation, host string
	err             error
}

func (e *hostError) Error() string { return fmt.Sprintf("%s network host %q", e.operation, e.host) }
func (e *hostError) Unwrap() error { return e.err }
