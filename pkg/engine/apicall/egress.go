package apicall

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/toggle"
)

type egressPolicy struct {
	blockedCIDRs    []*net.IPNet
	blockedHosts    map[string]struct{}
	allowedPrefixes []*url.URL
}

func loadEgressPolicy() (*egressPolicy, error) {
	return newEgressPolicy(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
}

func newEgressPolicy(blocklist, allowlist []string) (*egressPolicy, error) {
	p := &egressPolicy{blockedHosts: make(map[string]struct{})}
	for _, entry := range blocklist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			p.blockedHosts[normalizeHost(entry)] = struct{}{}
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in httpBlocklist: %w", entry, err)
		}
		p.blockedCIDRs = append(p.blockedCIDRs, ipNet)
	}
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		u, err := url.Parse(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid httpAllowlist URL %q: %w", entry, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("httpAllowlist entry %q must include scheme and host (e.g. https://api.example.com)", entry)
		}
		p.allowedPrefixes = append(p.allowedPrefixes, u)
	}
	return p, nil
}

func (p *egressPolicy) validateURL(rawURL string) error {
	if len(p.blockedHosts) == 0 && len(p.allowedPrefixes) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if len(p.allowedPrefixes) > 0 && !p.matchesAllowlist(u) {
		return fmt.Errorf("URL %q is not permitted: no matching allowlist entry", rawURL)
	}
	if _, blocked := p.blockedHosts[normalizeHost(u.Hostname())]; blocked {
		return fmt.Errorf("URL %q is blocked: hostname %q is on the blocklist", rawURL, u.Hostname())
	}
	return nil
}

func (p *egressPolicy) matchesAllowlist(reqURL *url.URL) bool {
	reqHost := normalizeHost(reqURL.Hostname())
	reqPort := effectivePort(reqURL)
	for _, entry := range p.allowedPrefixes {
		if reqURL.Scheme != entry.Scheme {
			continue
		}
		if normalizeHost(entry.Hostname()) != reqHost || effectivePort(entry) != reqPort {
			continue
		}
		if pathAllowed(reqURL.Path, entry.Path) {
			return true
		}
	}
	return false
}

func pathAllowed(reqPath, entryPath string) bool {
	if entryPath == "" || entryPath == "/" || reqPath == entryPath {
		return true
	}
	if !strings.HasPrefix(reqPath, entryPath) {
		return false
	}
	if strings.HasSuffix(entryPath, "/") {
		return true
	}
	return len(reqPath) > len(entryPath) && reqPath[len(entryPath)] == '/'
}

func (p *egressPolicy) dialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	blocked := p.blockedCIDRs
	base := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if cidr := containingCIDR(blocked, ip); cidr != nil {
				return nil, fmt.Errorf("connection to %s blocked: IP %s falls in blocked range %s", addr, ip, cidr)
			}
			return base.DialContext(ctx, network, addr)
		}
		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
		}
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if cidr := containingCIDR(blocked, ip); cidr != nil {
				return nil, fmt.Errorf("connection to %s blocked: resolved IP %s falls in blocked range %s", addr, ip, cidr)
			}
		}
		for _, ipStr := range ips {
			if net.ParseIP(ipStr) == nil {
				continue
			}
			return base.DialContext(ctx, network, net.JoinHostPort(ipStr, port))
		}
		return nil, fmt.Errorf("no usable addresses resolved for %s", host)
	}
}

func containingCIDR(cidrs []*net.IPNet, ip net.IP) *net.IPNet {
	for _, cidr := range cidrs {
		if cidr.Contains(ip) {
			return cidr
		}
	}
	return nil
}

func normalizeHost(h string) string {
	return strings.ToLower(strings.TrimRight(h, "."))
}

func effectivePort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	switch u.Scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}
