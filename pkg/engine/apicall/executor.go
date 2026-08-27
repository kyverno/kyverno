package apicall

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/logging"
	"github.com/kyverno/kyverno/pkg/toggle"
	"github.com/kyverno/kyverno/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Executor interface {
	Execute(context.Context, *kyvernov1.APICall) ([]byte, error)
}

type executor struct {
	logger logr.Logger
	name   string
	client ClientInterface
	config APICallConfiguration
}

func NewExecutor(
	logger logr.Logger,
	name string,
	client ClientInterface,
	apiCallConfig APICallConfiguration,
) *executor {
	return &executor{
		logger: logger,
		name:   name,
		client: client,
		config: apiCallConfig,
	}
}

func (a *executor) Execute(ctx context.Context, call *kyvernov1.APICall) ([]byte, error) {
	if call.URLPath != "" {
		return a.executeK8sAPICall(ctx, call.URLPath, call.Method, call.Data)
	}
	return a.executeServiceCall(ctx, call)
}

func (a *executor) executeK8sAPICall(ctx context.Context, path string, method kyvernov1.Method, data []kyvernov1.RequestData) ([]byte, error) {
	requestData, err := a.buildRequestData(data)
	if err != nil {
		return nil, err
	}
	jsonData, err := a.client.RawAbsPath(ctx, path, string(method), requestData)
	if err != nil {
		// Check for permission errors and provide clear error messages
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
			// StatusError contains detailed message about the permission issue
			// This surfaces RBAC errors that would otherwise only appear in debug logs
			return nil, fmt.Errorf("failed to %v resource with raw url: %s: permission denied: %v", method, path, err)
		}
		return nil, fmt.Errorf("failed to %v resource with raw url: %s: %v", method, path, err)
	}
	a.logger.V(4).Info("executed APICall", "name", a.name, "path", path, "method", method, "len", len(jsonData))
	return jsonData, nil
}

func (a *executor) executeServiceCall(ctx context.Context, apiCall *kyvernov1.APICall) ([]byte, error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.name)
	}

	policy, transport, err := getServiceHTTP()
	if err != nil {
		return nil, fmt.Errorf("failed to load HTTP blocklist/allowlist for APICall %s: %w", a.name, err)
	}
	if err := policy.validateURL(apiCall.Service.URL); err != nil {
		return nil, fmt.Errorf("failed to validate URL for APICall %s: %w", a.name, err)
	}

	client, err := a.buildHTTPClient(policy, transport, apiCall.Service)
	if err != nil {
		return nil, err
	}

	req, err := a.buildHTTPRequest(ctx, apiCall)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request for APICall %s: %w", a.name, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request for APICall %s: %w", a.name, err)
	}
	defer resp.Body.Close()
	var w http.ResponseWriter

	if a.config.maxAPICallResponseLength != 0 {
		resp.Body = http.MaxBytesReader(w, resp.Body, a.config.maxAPICallResponseLength)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, err := io.ReadAll(resp.Body)
		if err == nil {
			return nil, fmt.Errorf("HTTP %s: %s", resp.Status, string(b))
		}

		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			return nil, fmt.Errorf("response length must be less than max allowed response length of %d", a.config.maxAPICallResponseLength)
		} else {
			return nil, fmt.Errorf("failed to read data from APICall %s: %w", a.name, err)
		}
	}

	a.logger.V(4).Info("executed service APICall", "name", a.name, "len", len(body))
	return body, nil
}

func (a *executor) buildHTTPRequest(ctx context.Context, apiCall *kyvernov1.APICall) (*http.Request, error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service")
	}

	if apiCall.Method != "GET" && apiCall.Method != "POST" {
		return nil, fmt.Errorf("invalid request type %s for APICall %s", apiCall.Method, a.name)
	}

	var data io.Reader = nil
	if apiCall.Method == "POST" {
		var err error
		data, err = a.buildRequestData(apiCall.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to build request data for APICall %s: %w", a.name, err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, string(apiCall.Method), apiCall.Service.URL, data)
	if err != nil {
		return nil, fmt.Errorf("failed to build request for APICall %s: %w", a.name, err)
	}

	if err := a.addHTTPHeaders(req, apiCall.Service.Headers); err != nil {
		return nil, fmt.Errorf("failed to add headers for APICall %s: %w", a.name, err)
	}

	return req, nil
}

func (a *executor) addHTTPHeaders(req *http.Request, headers []kyvernov1.HTTPHeader) error {
	for _, header := range headers {
		req.Header.Add(header.Key, header.Value)
	}

	if req.Header.Get("Authorization") == "" {
		if token, ok := readScopedToken(); ok && token != "" {
			req.Header.Add("Authorization", "Bearer "+token)
		}
	}

	return nil
}

func (a *executor) buildHTTPClient(policy *egressPolicy, base http.RoundTripper, service *kyvernov1.ServiceCall) (*http.Client, error) {
	timeout := a.config.GetTimeout()
	if service == nil || service.CABundle == "" {
		return &http.Client{Transport: base, Timeout: timeout, CheckRedirect: checkServiceRedirect(policy)}, nil
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(service.CABundle)); !ok {
		return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall %s", a.name)
	}
	transport := newServiceTransport(policy)
	transport.TLSClientConfig = &tls.Config{RootCAs: caCertPool, MinVersion: tls.VersionTLS12}
	return &http.Client{
		Transport:     tracing.Transport(transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan)),
		Timeout:       timeout,
		CheckRedirect: checkServiceRedirect(policy),
	}, nil
}

func checkServiceRedirect(policy *egressPolicy) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return policy.validateURL(req.URL.String())
	}
}

var (
	serviceHTTPMu        sync.Mutex
	serviceHTTPPolicy    *egressPolicy
	serviceHTTPTransport http.RoundTripper
)

func getServiceHTTP() (*egressPolicy, http.RoundTripper, error) {
	serviceHTTPMu.Lock()
	defer serviceHTTPMu.Unlock()
	if serviceHTTPPolicy != nil {
		return serviceHTTPPolicy, serviceHTTPTransport, nil
	}
	policy, err := newEgressPolicy(toggle.HTTPBlocklist.Values(), toggle.HTTPAllowlist.Values())
	if err != nil {
		return nil, nil, err
	}
	serviceHTTPPolicy = policy
	serviceHTTPTransport = tracing.Transport(newServiceTransport(policy), otelhttp.WithFilter(tracing.RequestFilterIsInSpan))
	return serviceHTTPPolicy, serviceHTTPTransport, nil
}

func resetSharedServiceHTTP() {
	serviceHTTPMu.Lock()
	defer serviceHTTPMu.Unlock()
	serviceHTTPPolicy = nil
	serviceHTTPTransport = nil
}

func newServiceTransport(policy *egressPolicy) *http.Transport {
	var transport *http.Transport
	if base, ok := http.DefaultTransport.(*http.Transport); ok && base != nil {
		transport = base.Clone()
	} else {
		transport = &http.Transport{}
	}
	if policy != nil && len(policy.blockedCIDRs) > 0 {
		transport.DialContext = policy.dialContext()
		if base := transport.Proxy; base != nil {
			transport.Proxy = proxyAwareDestinationCheck(policy, base)
		}
	}
	return transport
}

func proxyAwareDestinationCheck(policy *egressPolicy, base func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	return func(req *http.Request) (*url.URL, error) {
		proxyURL, err := base(req)
		if err != nil || proxyURL == nil {
			return proxyURL, err
		}
		host := req.URL.Hostname()
		// With a proxy in use, DialContext only ever sees the proxy address, so the
		// blocklist has to be applied to the real destination here.
		//
		// The proxy resolves a hostname destination again, so unlike the direct-dial
		// path this check is not pinned to the address the connection ends up using: a
		// DNS rebinding window remains. That is accepted, because refusing every
		// proxied hostname breaks egress setups that work today, and because the
		// hostname blocklist and the allowlist both still apply.
		if err := policy.validateHostCIDRs(req.Context(), host); err != nil {
			if errors.Is(err, errHostUnresolvable) && len(policy.allowedPrefixes) > 0 {
				// Proxied egress commonly cannot resolve external names in-pod, since
				// that is the proxy's job. Allowing an unresolvable destination through
				// is safe only when an allowlist is configured, because every request
				// that reaches here has already been matched against it by validateURL
				// (on the initial URL, and again on each redirect hop).
				egressLogger().V(2).Info("proxied destination is not resolvable in-pod, skipping blocked CIDR check", "host", host, "proxy", proxyURL.Host, "reason", err.Error())
				return proxyURL, nil
			}
			if errors.Is(err, errHostUnresolvable) {
				return nil, fmt.Errorf("connection to %s via proxy %s blocked: the destination cannot be resolved in-pod, so it cannot be checked against the HTTP blocklist; use an IP address destination, exclude the host via NO_PROXY, or permit it explicitly with --httpAllowlist: %w", req.URL.Host, proxyURL.Host, err)
			}
			return nil, err
		}
		return proxyURL, nil
	}
}

func (a *executor) buildRequestData(data []kyvernov1.RequestData) (io.Reader, error) {
	dataMap := make(map[string]interface{})
	for _, d := range data {
		dataMap[d.Key] = d.Value
	}

	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(dataMap); err != nil {
		return nil, fmt.Errorf("failed to encode HTTP POST data %v for APICall %s: %w", dataMap, a.name, err)
	}

	return buffer, nil
}

// errHostUnresolvable marks a destination that could not be resolved in-pod, so that
// callers can tell "resolution failed" apart from "the destination is blocked". It is a
// match target for errors.Is only; unresolvableHostError keeps it out of the message so
// that a caller wrapping the error does not repeat itself.
var errHostUnresolvable = errors.New("destination is not resolvable")

type unresolvableHostError struct {
	host string
	err  error
}

func (e *unresolvableHostError) Error() string {
	return fmt.Sprintf("failed to resolve %s: %v", e.host, e.err)
}

func (e *unresolvableHostError) Unwrap() error { return e.err }

func (e *unresolvableHostError) Is(target error) bool { return target == errHostUnresolvable }

// lookupHost resolves a hostname. It is a variable so tests can force a specific answer
// or failure; resolution is not deterministic across platforms and CI environments.
var lookupHost = net.DefaultResolver.LookupHost

// egressLogger builds the logger lazily. A package-level logger would be created before
// logging.Setup runs and would discard everything written to it.
func egressLogger() logr.Logger {
	return logging.WithName("apicall-egress")
}

type egressPolicy struct {
	blockedCIDRs    []*net.IPNet
	blockedHosts    map[string]struct{}
	allowedPrefixes []*url.URL
}

func newEgressPolicy(blocklist, allowlist []string) (*egressPolicy, error) {
	p := &egressPolicy{blockedHosts: make(map[string]struct{})}
	for _, entry := range blocklist {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !strings.Contains(entry, "/") {
			p.blockedHosts[hostKey(entry)] = struct{}{}
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
	if len(p.allowedPrefixes) > 0 && !p.allowed(u) {
		return fmt.Errorf("URL %q is not permitted: no matching allowlist entry", rawURL)
	}
	if _, blocked := p.blockedHosts[hostKey(u.Hostname())]; blocked {
		return fmt.Errorf("URL %q is blocked: hostname %q is on the blocklist", rawURL, u.Hostname())
	}
	return nil
}

func (p *egressPolicy) allowed(u *url.URL) bool {
	host, port := hostKey(u.Hostname()), urlPort(u)
	for _, e := range p.allowedPrefixes {
		if u.Scheme == e.Scheme && hostKey(e.Hostname()) == host && urlPort(e) == port && pathAllowed(u.Path, e.Path) {
			return true
		}
	}
	return false
}

func pathAllowed(reqPath, entryPath string) bool {
	if entryPath == "" || entryPath == "/" {
		return true
	}
	entryPath = path.Clean(entryPath)
	reqPath = path.Clean("/" + reqPath)
	if reqPath == entryPath {
		return true
	}
	return strings.HasPrefix(reqPath, entryPath) && len(reqPath) > len(entryPath) && reqPath[len(entryPath)] == '/'
}

func (p *egressPolicy) validateHostCIDRs(ctx context.Context, host string) error {
	if host == "" || len(p.blockedCIDRs) == 0 {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if cidr := p.blockedCIDR(ip); cidr != nil {
			return fmt.Errorf("connection to %s blocked: IP %s falls in blocked range %s", host, ip, cidr)
		}
		return nil
	}
	ips, err := lookupHost(ctx, host)
	if err != nil {
		return &unresolvableHostError{host: host, err: err}
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if cidr := p.blockedCIDR(ip); cidr != nil {
			// The resolved address is deliberately left out of the error. It reaches the
			// caller through the admission denial, PolicyReports and Events, which would
			// turn a denial into an oracle for how an arbitrary name resolves from the
			// controller. The blocked range is operator-supplied, so naming it is safe.
			egressLogger().V(2).Info("blocked connection to resolved address", "host", host, "ip", ip.String(), "range", cidr.String())
			return fmt.Errorf("connection to %s blocked: it resolves into blocked range %s", host, cidr)
		}
	}
	return nil
}

// blockedCIDR returns the blocked range containing ip, or nil when there is none.
//
// net.IPNet.Contains normalizes IPv4-mapped IPv6 addresses through To4, so an IPv4 CIDR
// such as 169.254.169.254/32 already covers ::ffff:169.254.169.254. Three other IPv6
// spellings carry an IPv4 address that To4 does not recognise, so the embedded address is
// checked as a second candidate.
func (p *egressPolicy) blockedCIDR(ip net.IP) *net.IPNet {
	for _, candidate := range [2]net.IP{ip, embeddedIPv4(ip)} {
		if candidate == nil {
			continue
		}
		for _, cidr := range p.blockedCIDRs {
			if cidr.Contains(candidate) {
				return cidr
			}
		}
	}
	return nil
}

// embeddedIPv4 extracts the IPv4 address carried by an IPv6 address in a form that
// net.IP.To4 does not recognise: IPv4-compatible (::a.b.c.d), the NAT64 well-known prefix
// (64:ff9b::a.b.c.d), and 6to4 (2002:aabb:ccdd::). It returns nil when the address carries
// no embedded IPv4 address, or when To4 already handles it.
//
// Each of these is a distinct spelling of an address that an IPv4 CIDR on the blocklist is
// meant to cover. Without normalization, 64:ff9b::a9fe:a9fe reaches the metadata service
// even though 169.254.169.254/32 is blocked. NAT64 is the form that matters in practice,
// because it is standard in IPv6-only clusters; the other two are deprecated and need
// specific routing, so they are defense in depth.
//
// The IPv4-compatible branch also matches low IPv6 addresses that were never meant as an
// IPv4 encoding: ::1 yields 0.0.0.1, for example. That only ever widens the blocklist into
// reserved space, so it is left as is.
func embeddedIPv4(ip net.IP) net.IP {
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

const (
	// dialTimeout bounds resolution and every connection attempt together, the way
	// net.Dialer's own Timeout does for a caller that hands it a hostname.
	dialTimeout = 30 * time.Second
	// fallbackDelay is the head start the preferred address family gets before the other
	// one is tried in parallel (RFC 6555). It matches net.Dialer's default.
	fallbackDelay = 300 * time.Millisecond
)

func (p *egressPolicy) dialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	// No per-attempt Timeout. The deadline below spans resolution and every attempt
	// together; setting one here as well would let a host with many answers stretch the
	// total to dialTimeout per address.
	base := &net.Dialer{KeepAlive: 30 * time.Second}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// One deadline covering resolution and all connection attempts, which is what the
		// stock dialer gives a caller. Without it, a name answering with many black-holed
		// addresses holds an admission request for resolution plus dialTimeout per
		// address. A shorter deadline already on ctx still wins.
		ctx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()
		if ip := net.ParseIP(host); ip != nil {
			if cidr := p.blockedCIDR(ip); cidr != nil {
				return nil, fmt.Errorf("connection to %s blocked: IP %s falls in blocked range %s", addr, ip, cidr)
			}
			return dialTCP(ctx, base, network, addr)
		}
		ips, err := lookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve %s: %w", host, err)
		}
		targets := make([]net.IP, 0, len(ips))
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if cidr := p.blockedCIDR(ip); cidr != nil {
				// See validateHostCIDRs: the resolved address stays out of the error so
				// the denial does not act as a DNS resolution oracle.
				egressLogger().V(2).Info("blocked connection to resolved address", "addr", addr, "ip", ip.String(), "range", cidr.String())
				return nil, fmt.Errorf("connection to %s blocked: %s resolves into blocked range %s", addr, host, cidr)
			}
			targets = append(targets, ip)
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("no usable addresses resolved for %s", host)
		}
		// Every dial below targets an address that was just validated rather than the
		// hostname, so a second resolution cannot substitute a different one (DNS
		// rebinding).
		//
		// Split by family and race the two groups the way net.Dialer does, rather than
		// walking one flat list. Trying every address of the preferred family first makes
		// a dual-stack Service whose AAAA is present but not listening wait out the whole
		// IPv6 side before reaching IPv4; the stock dialer starts the other family after
		// fallbackDelay instead. lookupHost returns addresses in RFC 6724 preference
		// order, so the first one names the preferred family.
		preferIPv4 := targets[0].To4() != nil
		var primary, fallback []net.IP
		for _, ip := range targets {
			if (ip.To4() != nil) == preferIPv4 {
				primary = append(primary, ip)
			} else {
				fallback = append(fallback, ip)
			}
		}
		if len(fallback) == 0 {
			return dialSeries(ctx, base, network, port, primary)
		}
		return dialRace(ctx, base, network, port, primary, fallback)
	}
}

// dialTCP performs one connection attempt. It is a variable so tests can observe the
// context handed to each attempt, which is what proves the deadline above is shared rather
// than per-address; a test that relies on unreachable addresses instead passes vacuously on
// any host where those addresses fail fast.
var dialTCP = func(ctx context.Context, d *net.Dialer, network, addr string) (net.Conn, error) {
	return d.DialContext(ctx, network, addr)
}

// dialSeries tries each address in turn and returns the first connection that succeeds.
func dialSeries(ctx context.Context, d *net.Dialer, network, port string, ips []net.IP) (net.Conn, error) {
	var lastErr error
	for _, ip := range ips {
		conn, err := dialTCP(ctx, d, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	if lastErr == nil {
		lastErr = errors.New("no addresses to dial")
	}
	return nil, lastErr
}

type dialOutcome struct {
	conn      net.Conn
	err       error
	isPrimary bool
}

// dialRace runs the two address families concurrently, giving the preferred one a
// fallbackDelay head start, and returns the first connection to succeed (RFC 6555).
func dialRace(ctx context.Context, d *net.Dialer, network, port string, primary, fallback []net.IP) (net.Conn, error) {
	ctx, cancel := context.WithCancel(ctx)
	// Cancelling on return tears down the losing attempt. A connection already returned is
	// unaffected: net.Dialer does not tie a live conn to the dial context.
	defer cancel()

	// Buffered, so a loser that finishes after we have returned never blocks on send.
	outcomes := make(chan dialOutcome, 2)
	race := func(ips []net.IP, isPrimary bool, delay time.Duration) {
		go func() {
			if delay > 0 {
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-timer.C:
				case <-ctx.Done():
					outcomes <- dialOutcome{err: ctx.Err(), isPrimary: isPrimary}
					return
				}
			}
			conn, err := dialSeries(ctx, d, network, port, ips)
			outcomes <- dialOutcome{conn: conn, err: err, isPrimary: isPrimary}
		}()
	}
	race(primary, true, 0)
	race(fallback, false, fallbackDelay)

	var firstErr error
	for i := 0; i < 2; i++ {
		out := <-outcomes
		if out.err == nil {
			// Drain the sibling so a connection that lands after this point is closed
			// rather than leaked.
			if remaining := 1 - i; remaining > 0 {
				go func(n int) {
					for j := 0; j < n; j++ {
						if late := <-outcomes; late.conn != nil {
							late.conn.Close()
						}
					}
				}(remaining)
			}
			return out.conn, nil
		}
		// Prefer the primary family's error: it describes the address the caller would
		// have reached without this guard.
		if firstErr == nil || out.isPrimary {
			firstErr = out.err
		}
	}
	return nil, firstErr
}

func hostKey(h string) string {
	return strings.ToLower(strings.TrimRight(h, "."))
}

func urlPort(u *url.URL) string {
	if p := u.Port(); p != "" {
		return p
	}
	if u.Scheme == "https" {
		return "443"
	}
	if u.Scheme == "http" {
		return "80"
	}
	return ""
}
