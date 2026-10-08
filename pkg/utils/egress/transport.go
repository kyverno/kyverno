package egress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"github.com/kyverno/kyverno/pkg/logging"
)

// DialTimeout preserves the registry SDK's five-second dial budget across DNS
// resolution and all connection attempts together. A shorter caller deadline wins.
const DialTimeout = 5 * time.Second

type dialPlanKey struct{}

type dialPlan struct {
	target, proxyAddress string
	targetIPs, proxyIPs  []netip.Addr
	proxy                *url.URL
	deadline             time.Time
}

// Transport validates each request, including redirects and requests reusing a
// connection. Dial information is scoped to that request so a trusted private
// proxy never grants a direct request permission to reach private destinations.
type Transport struct {
	base   *http.Transport
	policy *Policy
	proxy  func(*http.Request) (*url.URL, error)
}

// WrapTransport clones base, preserving TLS and pooling settings. A proxy is
// trusted to resolve the downstream destination, but its target is still locally
// resolved and validated on every request. Direct and proxy connections are
// pinned to their validated addresses; exemptions never skip these checks.
func (p *Policy) WrapTransport(base *http.Transport) *Transport {
	if base == nil {
		base = &http.Transport{}
	}
	transport := &Transport{base: base.Clone(), policy: p, proxy: base.Proxy}
	baseDial := transport.base.DialContext
	if baseDial == nil {
		baseDial = (&net.Dialer{KeepAlive: 30 * time.Second}).DialContext
	}
	transport.base.Proxy = func(request *http.Request) (*url.URL, error) {
		plan, ok := request.Context().Value(dialPlanKey{}).(*dialPlan)
		if !ok {
			return nil, fmt.Errorf("missing validated network destination")
		}
		return plan.proxy, nil
	}
	transport.base.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		plan, ok := ctx.Value(dialPlanKey{}).(*dialPlan)
		if !ok {
			return nil, fmt.Errorf("missing validated network destination")
		}
		ctx, cancel := context.WithDeadline(ctx, plan.deadline)
		defer cancel()
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid network address: %w", err)
		}
		var addresses []netip.Addr
		switch {
		case plan.proxy != nil && address == plan.proxyAddress:
			addresses = plan.proxyIPs
		case plan.proxy == nil && address == plan.target:
			addresses = plan.targetIPs
		default:
			return nil, fmt.Errorf("unvalidated connection to network host %q", host)
		}
		ips := make([]net.IP, len(addresses))
		for i, address := range addresses {
			ips[i] = net.IP(address.AsSlice())
		}
		connection, err := DialAddresses(ctx, baseDial, network, port, ips)
		if err != nil {
			logging.WithName("egress").V(2).Info("network connection failed", "host", host, "error", err.Error())
			return nil, &hostError{operation: "failed to connect to", host: host, err: err}
		}
		return connection, nil
	}
	// Custom TLS dialers bypass DialContext, so TLS must use the transport's
	// normal handshake after our validated connection has been established.
	transport.base.DialTLSContext = nil
	transport.base.DialTLS = nil
	return transport
}

func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	handedOff := false
	defer func() {
		if !handedOff && request.Body != nil {
			_ = request.Body.Close()
		}
	}()
	if request.URL == nil || (request.URL.Scheme != "http" && request.URL.Scheme != "https") {
		return nil, fmt.Errorf("network destination must use HTTP or HTTPS")
	}
	// WriteProxy uses Host for the absolute request target and URL.Opaque can
	// override that target entirely. Neither may select an unchecked destination.
	if request.URL.Opaque != "" {
		return nil, fmt.Errorf("opaque network destinations are not permitted: %w", ErrAddressBlocked)
	}
	if request.Host != "" {
		expected, err := normalizedAuthority(request.URL)
		if err != nil {
			return nil, err
		}
		actual, err := normalizedAuthority(&url.URL{Scheme: request.URL.Scheme, Host: request.Host})
		if err != nil {
			return nil, err
		}
		if actual != expected {
			return nil, fmt.Errorf("network request Host does not match URL authority: %w", ErrAddressBlocked)
		}
	}
	deadline := time.Now().Add(DialTimeout)
	if callerDeadline, ok := request.Context().Deadline(); ok && callerDeadline.Before(deadline) {
		deadline = callerDeadline
	}
	ctx, cancel := context.WithDeadline(request.Context(), deadline)
	defer cancel()
	targetIPs, err := t.policy.ResolveAndValidate(ctx, request.URL.Hostname())
	if err != nil {
		return nil, err
	}
	plan := &dialPlan{target: canonicalAddress(request.URL), targetIPs: targetIPs, deadline: deadline}
	if t.proxy != nil {
		plan.proxy, err = t.proxy(request)
		if err != nil {
			return nil, err
		}
	}
	if plan.proxy != nil {
		// Clone operator configuration before storing it with a request.
		proxy := *plan.proxy
		plan.proxy = &proxy
		plan.proxyAddress = canonicalAddress(plan.proxy)
		plan.proxyIPs, err = t.policy.resolveAndValidate(ctx, plan.proxy.Hostname(), true)
		if err != nil {
			return nil, err
		}
	}
	// The DNS timeout must not cancel the response body. The original request
	// context controls the HTTP exchange; only the dial inherits this deadline.
	guarded := request.Clone(context.WithValue(request.Context(), dialPlanKey{}, plan))
	handedOff = true
	return t.base.RoundTrip(guarded)
}

// CloseIdleConnections releases idle pooled connections without interrupting requests.
func (t *Transport) CloseIdleConnections() { t.base.CloseIdleConnections() }

func canonicalAddress(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func normalizedAuthority(u *url.URL) (string, error) {
	host, err := normalizeHost(u.Host)
	if err != nil {
		return "", fmt.Errorf("invalid network authority: %w", err)
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	} else {
		// normalizeHost has already validated the numeric port and its range.
		number, _ := strconv.Atoi(port)
		port = strconv.Itoa(number)
	}
	return net.JoinHostPort(host, port), nil
}
