package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"time"

	"github.com/kyverno/kyverno/pkg/logging"
)

// DialTimeout matches the registry SDK's five-second dial budget. It also bounds the DNS lookup
// RoundTrip makes before handing the request on; a shorter caller deadline wins.
const DialTimeout = 5 * time.Second

var errMissingRoute = errors.New("network request did not pass the egress check")

type routeKey struct{}

// route carries the proxy decision RoundTrip checked down to the inner transport, so its proxy
// and dial hooks act on that decision rather than making their own.
type route struct{ proxy *url.URL }

// Transport checks each request, including redirects and token challenges, then passes it to an
// ordinary http.Transport.
type Transport struct {
	inner  *http.Transport
	policy *Policy
	proxy  func(*http.Request) (*url.URL, error)
}

// WrapTransport clones base, keeping its TLS, proxy and pooling settings. The clone resolves the
// name again when it dials, so the address each direct connection reaches is checked as well: a
// DNS answer that changed after RoundTrip's check (rebinding) is still refused. A proxied request
// only dials the proxy, which is exempt. The proxy resolves the target itself, so the denied-IP
// list cannot bind where it connects; restricting that is the proxy's job.
func (p *Policy) WrapTransport(base *http.Transport) *Transport {
	if base == nil {
		base = &http.Transport{}
	}
	inner := base.Clone()
	dial := inner.DialContext
	if dial == nil {
		dial = (&net.Dialer{Timeout: DialTimeout, KeepAlive: 30 * time.Second}).DialContext
	}
	inner.Proxy = func(request *http.Request) (*url.URL, error) {
		route, ok := request.Context().Value(routeKey{}).(route)
		if !ok {
			return nil, errMissingRoute
		}
		return route.proxy, nil
	}
	inner.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		route, ok := ctx.Value(routeKey{}).(route)
		if !ok {
			return nil, errMissingRoute
		}
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid network address: %w", err)
		}
		conn, err := dial(ctx, network, address)
		if err != nil {
			return nil, &hostError{operation: "failed to connect to", host: host, err: err}
		}
		if route.proxy != nil {
			return conn, nil
		}
		// Connections that are not TCP, such as test pipes, carry no address to check.
		if remote, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			address, _ := netip.AddrFromSlice(remote.IP)
			if err := p.checkAddress(host, address.Unmap()); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
		return conn, nil
	}
	// A custom TLS dialer would bypass DialContext and the check above.
	inner.DialTLSContext = nil
	inner.DialTLS = nil
	return &Transport{inner: inner, policy: p, proxy: base.Proxy}
}

func (t *Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	route, err := t.check(request)
	if err != nil {
		if request.Body != nil {
			_ = request.Body.Close()
		}
		return nil, err
	}
	return t.inner.RoundTrip(request.Clone(context.WithValue(request.Context(), routeKey{}, route)))
}

// CloseIdleConnections releases idle pooled connections without interrupting requests.
func (t *Transport) CloseIdleConnections() {
	t.inner.CloseIdleConnections()
}

func (t *Transport) check(request *http.Request) (route, error) {
	if request.URL == nil || (request.URL.Scheme != "http" && request.URL.Scheme != "https") {
		return route{}, fmt.Errorf("network destination must use HTTP or HTTPS")
	}
	// WriteProxy uses Host for the absolute request target and URL.Opaque can
	// override that target entirely. Neither may select an unchecked destination.
	if request.URL.Opaque != "" {
		return route{}, fmt.Errorf("opaque network destinations are not permitted: %w", ErrAddressBlocked)
	}
	if request.Host != "" {
		expected, err := normalizedAuthority(request.URL)
		if err != nil {
			return route{}, err
		}
		actual, err := normalizedAuthority(&url.URL{Scheme: request.URL.Scheme, Host: request.Host})
		if err != nil {
			return route{}, err
		}
		if actual != expected {
			return route{}, fmt.Errorf("network request Host does not match URL authority: %w", ErrAddressBlocked)
		}
	}
	var checked route
	if t.proxy != nil {
		proxy, err := t.proxy(request)
		if err != nil {
			return route{}, err
		}
		checked.proxy = proxy
	}
	ctx, cancel := context.WithTimeout(request.Context(), DialTimeout)
	defer cancel()
	if _, err := t.policy.ResolveAndValidate(ctx, request.URL.Hostname()); err != nil {
		// A registry only the proxy can resolve is not evidence of a bad destination, so it is
		// left to the proxy. The denied-host check has already run, and a literal address never
		// reaches the resolver.
		if checked.proxy == nil || !dnsNotFound(err) {
			return route{}, err
		}
		logging.WithName("egress").V(2).Info("network destination left to the proxy's resolver", "host", request.URL.Hostname())
	}
	return checked, nil
}

// dnsNotFound reports whether the local resolver simply does not know the name, as opposed to
// the policy having rejected it. Only the former may be deferred to a proxy: ErrAddressBlocked
// and every other resolver failure still fail closed.
func dnsNotFound(err error) bool {
	if errors.Is(err, ErrAddressBlocked) {
		return false
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
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
