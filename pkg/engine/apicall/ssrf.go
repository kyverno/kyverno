package apicall

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/kyverno/kyverno/pkg/toggle"
	celhttp "github.com/kyverno/sdk/extensions/cel/libs/http"
)

// validateServiceCallURL enforces the same --httpBlocklist/--httpAllowlist rules used by
// CEL http.Get/Post before a legacy context.apiCall.service request is dialed.
// Hostname and allowlist checks run here; CIDR blocking is enforced in the HTTP client's
// DialContext (see buildHTTPClient).
func validateServiceCallURL(rawURL string) error {
	// Ensure flag values parse the same way as the CEL path.
	if _, err := celhttp.NewHTTPWithBlocklist(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values()); err != nil {
		return fmt.Errorf("invalid HTTP filter configuration: %w", err)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid service URL %q: must include scheme and host", rawURL)
	}

	blocklist := toggle.HTTPBlocklist.Values()
	allowlist := toggle.HTTPAllowlist.Values()

	allowedURLPrefixes, err := parseAllowlist(allowlist)
	if err != nil {
		return err
	}
	if len(allowedURLPrefixes) > 0 && !matchesAllowlist(u, allowedURLPrefixes) {
		return fmt.Errorf("URL %q is not permitted: no matching allowlist entry", rawURL)
	}

	blockedHosts := make(map[string]struct{})
	for _, entry := range blocklist {
		entry = strings.TrimSpace(entry)
		if entry == "" || strings.Contains(entry, "/") {
			continue
		}
		blockedHosts[normalizeHost(entry)] = struct{}{}
	}
	host := u.Hostname()
	if _, blocked := blockedHosts[normalizeHost(host)]; blocked {
		return fmt.Errorf("URL %q is blocked: hostname %q is on the blocklist", rawURL, host)
	}
	return nil
}

func parseAllowlist(allowlist []string) ([]*url.URL, error) {
	var allowed []*url.URL
	for _, entry := range allowlist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		u, err := url.Parse(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid allowlist URL %q: %w", entry, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("allowlist entry %q must include scheme and host", entry)
		}
		allowed = append(allowed, u)
	}
	return allowed, nil
}

func parseBlockedCIDRs(blocklist []string) ([]*net.IPNet, error) {
	var blocked []*net.IPNet
	for _, entry := range blocklist {
		entry = strings.TrimSpace(entry)
		if entry == "" || !strings.Contains(entry, "/") {
			continue
		}
		_, ipNet, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q in blocklist: %w", entry, err)
		}
		blocked = append(blocked, ipNet)
	}
	return blocked, nil
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

func matchesAllowlist(reqURL *url.URL, allowedURLPrefixes []*url.URL) bool {
	reqHost := normalizeHost(reqURL.Hostname())
	reqPort := effectivePort(reqURL)
	for _, entry := range allowedURLPrefixes {
		if reqURL.Scheme != entry.Scheme {
			continue
		}
		if normalizeHost(entry.Hostname()) != reqHost || effectivePort(entry) != reqPort {
			continue
		}
		entryPath := entry.Path
		if entryPath == "" || entryPath == "/" {
			return true
		}
		if reqURL.Path == entryPath {
			return true
		}
		if strings.HasPrefix(reqURL.Path, entryPath) {
			if entryPath[len(entryPath)-1] == '/' {
				return true
			}
			if len(reqURL.Path) > len(entryPath) && reqURL.Path[len(entryPath)] == '/' {
				return true
			}
		}
	}
	return false
}

// secureDialContext mirrors github.com/kyverno/sdk/.../http.secureDialContext so
// ServiceCall clients enforce CIDR blocklists at dial time (including DNS-rebinding).
func secureDialContext(blockedCIDRs []*net.IPNet) func(ctx context.Context, network, addr string) (net.Conn, error) {
	base := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			for _, cidr := range blockedCIDRs {
				if cidr.Contains(ip) {
					return nil, fmt.Errorf("connection to %s blocked: IP %s falls in blocked range %s", addr, ip, cidr)
				}
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
			for _, cidr := range blockedCIDRs {
				if cidr.Contains(ip) {
					return nil, fmt.Errorf("connection to %s blocked: resolved IP %s falls in blocked range %s", addr, ip, cidr)
				}
			}
		}
		for _, ipStr := range ips {
			if net.ParseIP(ipStr) == nil {
				continue
			}
			return base.DialContext(ctx, network, net.JoinHostPort(ipStr, port))
		}
		return nil, fmt.Errorf("no usable addresses resolved for %s", addr)
	}
}

func applySSRFDialContext(transport *http.Transport) error {
	blockedCIDRs, err := parseBlockedCIDRs(toggle.HTTPBlocklist.Values())
	if err != nil {
		return err
	}
	if len(blockedCIDRs) > 0 {
		transport.DialContext = secureDialContext(blockedCIDRs)
	}
	return nil
}
