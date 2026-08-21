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
		if err := policy.validateHostCIDRs(req.Context(), req.URL.Hostname()); err != nil {
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
			if ip := net.ParseIP(entry); ip != nil {
				p.blockedCIDRs = append(p.blockedCIDRs, ipToCIDR(ip))
			} else {
				p.blockedHosts[hostKey(entry)] = struct{}{}
			}
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
	blockedIP := func(ip net.IP) *net.IPNet {
		for _, cidr := range p.blockedCIDRs {
			if cidr.Contains(ip) {
				return cidr
			}
		}
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		if cidr := blockedIP(ip); cidr != nil {
			return fmt.Errorf("connection to %s blocked: IP %s falls in blocked range %s", host, ip, cidr)
		}
		return nil
	}
	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to resolve %s: %w", host, err)
	}
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			continue
		}
		if cidr := blockedIP(ip); cidr != nil {
			return fmt.Errorf("connection to %s blocked: resolved IP %s falls in blocked range %s", host, ip, cidr)
		}
	}
	return nil
}

func (p *egressPolicy) dialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	blocked := p.blockedCIDRs
	base := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
	blockedIP := func(ip net.IP) *net.IPNet {
		for _, cidr := range blocked {
			if cidr.Contains(ip) {
				return cidr
			}
		}
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if ip := net.ParseIP(host); ip != nil {
			if cidr := blockedIP(ip); cidr != nil {
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
			if cidr := blockedIP(ip); cidr != nil {
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

func ipToCIDR(ip net.IP) *net.IPNet {
	if v4 := ip.To4(); v4 != nil {
		return &net.IPNet{IP: v4, Mask: net.CIDRMask(32, 32)}
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
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
