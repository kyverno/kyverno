package apicall

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
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

// validateServiceURL enforces the operator-configured blocklist and allowlist.
// CIDR blocklist entries are skipped here and checked at dial time by secureDialContext.
func validateServiceURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}

	// Reject URLs with userinfo — prevents bypass via https://allowed.com@evil.com/
	if u.User != nil {
		return fmt.Errorf("URL %q is not permitted: userinfo in URL is not allowed", rawURL)
	}

	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))

	if allowlist := toggle.HTTPAllowlist.Values(); len(allowlist) > 0 {
		allowed := false
		for _, entry := range allowlist {
			entryURL, err := url.Parse(entry)
			if err != nil {
				continue
			}
			if strings.EqualFold(u.Scheme, entryURL.Scheme) &&
				strings.EqualFold(u.Hostname(), entryURL.Hostname()) &&
				strings.HasPrefix(u.Path, entryURL.Path) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("URL %q is not permitted: no matching allowlist entry", rawURL)
		}
	}

	for _, entry := range toggle.HTTPBlocklist.Values() {
		if _, _, err := net.ParseCIDR(entry); err == nil {
			continue
		}
		if strings.ToLower(strings.TrimSuffix(entry, ".")) == host {
			return fmt.Errorf("URL %q is blocked: hostname %q is on the blocklist", rawURL, host)
		}
	}

	return nil
}

// secureDialContext returns a DialContext that checks resolved IPs against CIDR and IP-literal
// blocklist entries before connecting. IP literals are treated as single-host networks (/32 or
// /128). Pinning to the resolved IP prevents DNS-rebinding. Returns nil if no CIDR or IP-literal
// entries are present.
func secureDialContext(blocklist []string) func(ctx context.Context, network, addr string) (net.Conn, error) {
	var cidrs []*net.IPNet
	for _, entry := range blocklist {
		if _, ipNet, err := net.ParseCIDR(entry); err == nil {
			cidrs = append(cidrs, ipNet)
			continue
		}
		if ip := net.ParseIP(entry); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				cidrs = append(cidrs, &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)})
			} else {
				cidrs = append(cidrs, &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)})
			}
		}
	}
	if len(cidrs) == 0 {
		return nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host %q: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no addresses resolved for host %q", host)
		}
		for _, ipStr := range ips {
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			for _, cidr := range cidrs {
				if cidr.Contains(ip) {
					return nil, fmt.Errorf("host %q resolves to blocked address %s (%s)", host, ip, cidr)
				}
			}
		}
		d := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
		var lastErr error
		for _, ipStr := range ips {
			conn, err := d.DialContext(ctx, network, net.JoinHostPort(ipStr, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		return nil, lastErr
	}
}

func (a *executor) executeServiceCall(ctx context.Context, apiCall *kyvernov1.APICall) ([]byte, error) {
	if apiCall.Service == nil {
		return nil, fmt.Errorf("missing service for APICall %s", a.name)
	}

	if err := validateServiceURL(apiCall.Service.URL); err != nil {
		return nil, fmt.Errorf("APICall %s: %w", a.name, err)
	}

	client, err := a.buildHTTPClient(apiCall.Service)
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

func (a *executor) buildHTTPClient(service *kyvernov1.ServiceCall) (*http.Client, error) {
	timeout := a.config.GetTimeout()
	dialCtx := secureDialContext(toggle.HTTPBlocklist.Values())

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if dialCtx != nil {
		transport.DialContext = dialCtx
	}

	if service != nil && service.CABundle != "" {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(service.CABundle)); !ok {
			return nil, fmt.Errorf("failed to parse PEM CA bundle for APICall %s", a.name)
		}
		transport.TLSClientConfig = &tls.Config{
			RootCAs:    caCertPool,
			MinVersion: tls.VersionTLS12,
		}
	}

	return &http.Client{
		Transport: tracing.Transport(transport, otelhttp.WithFilter(tracing.RequestFilterIsInSpan)),
		Timeout:   timeout,
	}, nil
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
