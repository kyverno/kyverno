package egress

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kyverno/kyverno/pkg/logging"
)

// DialTimeout preserves the registry SDK's five-second dial budget across DNS
// resolution and all connection attempts together. A shorter caller deadline wins.
const DialTimeout = 5 * time.Second

// Bound pool growth when DNS answers change or policies use many registries.
const maxDestinationPools = 64

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
	mu     sync.Mutex
	pools  map[destinationKey]*list.Element
	lru    list.List
}

type destinationKey struct {
	targetIPs string
	proxyIPs  string
}

type destinationPool struct {
	key       destinationKey
	transport *http.Transport
	retired   atomic.Bool
}

// WrapTransport clones base, preserving TLS and pooling settings. Direct
// connections pin target addresses; proxied connections pin only proxy addresses.
// The target is still resolved and validated locally on every request, but the
// proxy owns downstream DNS and routing, which are not pinned by this transport.
// Exemptions never skip these checks.
func (p *Policy) WrapTransport(base *http.Transport) *Transport {
	if base == nil {
		base = &http.Transport{}
	}
	transport := &Transport{base: base.Clone(), policy: p, proxy: base.Proxy, pools: make(map[destinationKey]*list.Element)}
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
	plan := &dialPlan{target: canonicalAddress(request.URL), deadline: deadline}
	// Resolve the proxy decision first: whether a target has to be resolvable here depends
	// on it.
	var err error
	if t.proxy != nil {
		plan.proxy, err = t.proxy(request)
		if err != nil {
			return nil, err
		}
	}
	plan.targetIPs, err = t.policy.ResolveAndValidate(ctx, request.URL.Hostname())
	if err != nil {
		// A proxy owns downstream DNS, so a registry only it can resolve is not evidence of
		// a bad destination, and failing here would break those deployments in every mode.
		// Anything that does resolve is still validated, and a literal address never reaches
		// the resolver at all, so a blocked destination cannot be reached this way.
		if plan.proxy == nil || !dnsNotFound(err) {
			return nil, err
		}
		logging.WithName("egress").V(2).Info("network destination left to the proxy's resolver", "host", request.URL.Hostname())
		plan.targetIPs = nil
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
	pool := t.destinationPool(plan)
	response, err := pool.transport.RoundTrip(guarded)
	if err != nil {
		pool.closeIfRetired()
		return nil, err
	}
	response.Body = &pooledResponseBody{ReadCloser: response.Body, pool: pool}
	return response, nil
}

// CloseIdleConnections releases idle pooled connections without interrupting requests.
func (t *Transport) CloseIdleConnections() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, element := range t.pools {
		element.Value.(*destinationPool).transport.CloseIdleConnections()
	}
}

// Each pool sees only one validated address set. http.Transport continues to
// separate authorities, proxies and TLS sessions within that pool, preserving
// Host/SNI and reuse without connecting to a previous request's stale address.
//
// A proxied request only ever dials the proxy, so only the proxy's addresses can
// go stale and only they may select the pool. Including the target would tear
// down a healthy proxy connection whenever any registry's DNS answer changed,
// and a fleet of registries behind one proxy would churn the bounded cache,
// evicting pools that are still useful. The two key shapes cannot collide:
// validation rejects an empty address set, so each shape leaves exactly one
// field non-empty and a direct key can never equal a proxied one.
func (t *Transport) destinationPool(plan *dialPlan) *destinationPool {
	key := destinationKey{targetIPs: addressSetKey(plan.targetIPs)}
	if plan.proxy != nil {
		key = destinationKey{proxyIPs: addressSetKey(plan.proxyIPs)}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if element := t.pools[key]; element != nil {
		t.lru.MoveToFront(element)
		return element.Value.(*destinationPool)
	}
	pool := &destinationPool{key: key, transport: t.base.Clone()}
	t.pools[key] = t.lru.PushFront(pool)
	if t.lru.Len() > maxDestinationPools {
		oldest := t.lru.Back()
		retired := oldest.Value.(*destinationPool)
		delete(t.pools, retired.key)
		t.lru.Remove(oldest)
		retired.retired.Store(true)
		retired.transport.CloseIdleConnections()
	}
	return pool
}

func addressSetKey(addresses []netip.Addr) string {
	values := make([]string, len(addresses))
	for i, address := range addresses {
		values[i] = address.String()
	}
	slices.Sort(values)
	return strings.Join(slices.Compact(values), ",")
}

func (p *destinationPool) closeIfRetired() {
	if p.retired.Load() {
		p.transport.CloseIdleConnections()
	}
}

type pooledResponseBody struct {
	io.ReadCloser
	pool *destinationPool
}

func (b *pooledResponseBody) Close() error {
	err := b.ReadCloser.Close()
	// An evicted pool may still have active requests. Close their connections
	// when they become idle rather than interrupting the response body.
	b.pool.closeIfRetired()
	return err
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
