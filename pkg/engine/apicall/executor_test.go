package apicall

import (
	"context"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"gotest.tools/v3/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockClientWithError struct {
	err error
}

func (c *mockClientWithError) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return nil, c.err
}

func Test_ExecuteK8sAPICall_ForbiddenError(t *testing.T) {
	// Create a Forbidden error similar to what K8s API returns
	forbiddenErr := apierrors.NewForbidden(
		schema.GroupResource{Group: "storage.k8s.io", Resource: "volumeattachments"},
		"",
		errors.New("User \"system:serviceaccount:kyverno:kyverno-admission-controller\" cannot list resource \"volumeattachments\" in API group \"storage.k8s.io\" at the cluster scope"),
	)

	client := &mockClientWithError{err: forbiddenErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig)

	call := &kyvernov1.APICall{
		URLPath: "/apis/storage.k8s.io/v1/volumeattachments",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "permission denied")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	assert.ErrorContains(t, err, "/apis/storage.k8s.io/v1/volumeattachments")
}

func Test_ExecuteK8sAPICall_UnauthorizedError(t *testing.T) {
	// Create an Unauthorized error similar to what K8s API returns
	unauthorizedErr := apierrors.NewUnauthorized("access denied")

	client := &mockClientWithError{err: unauthorizedErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig)

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "permission denied")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	assert.ErrorContains(t, err, "/api/v1/namespaces")
}

func Test_ExecuteK8sAPICall_OtherError(t *testing.T) {
	// Create a NotFound error (non-permission error)
	notFoundErr := apierrors.NewNotFound(
		schema.GroupResource{Group: "", Resource: "configmaps"},
		"test-config",
	)

	client := &mockClientWithError{err: notFoundErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig)

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces/default/configmaps/test-config",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	// Should NOT contain "permission denied" prefix for non-permission errors
	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	// Verify it doesn't have permission denied prefix
	errMsg := err.Error()
	assert.Check(t, !contains(errMsg, "permission denied"))
}

func Test_ExecuteK8sAPICall_GenericError(t *testing.T) {
	// Generic error that's not a K8s API error
	genericErr := errors.New("connection timeout")

	client := &mockClientWithError{err: genericErr}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig)

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/pods",
		Method:  "GET",
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.Check(t, err != nil)
	assert.ErrorContains(t, err, "connection timeout")
	assert.ErrorContains(t, err, "failed to GET resource with raw url")
	// Verify it doesn't have permission denied prefix
	errMsg := err.Error()
	assert.Check(t, !contains(errMsg, "permission denied"))
}

func Test_ExecuteK8sAPICall_Success(t *testing.T) {
	client := &mockClient{}
	executor := NewExecutor(logr.Discard(), "test-call", client, apiConfig)

	call := &kyvernov1.APICall{
		URLPath: "/api/v1/namespaces",
		Method:  "GET",
	}

	data, err := executor.Execute(context.TODO(), call)
	assert.NilError(t, err)
	assert.Equal(t, string(data), "{}")
}

func Test_ExecuteServiceCall_AllowsMissingScopedTokenWhenAuthorizationMissing(t *testing.T) {
	withEmptyEgressBlocklist(t)
	missingTokenPath := scopedTokenPath + ".missing"
	oldPath := scopedTokenPath
	scopedTokenPath = missingTokenPath
	t.Cleanup(func() {
		scopedTokenPath = oldPath
	})

	var gotAuth string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	executor := NewExecutor(logr.Discard(), "test-call", &mockClient{}, apiConfig)
	call := &kyvernov1.APICall{
		Method: "GET",
		Service: &kyvernov1.ServiceCall{
			URL: s.URL,
		},
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.NilError(t, err)
	assert.Equal(t, gotAuth, "")
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func withEmptyEgressBlocklist(t *testing.T) {
	t.Helper()
	assert.NilError(t, toggle.HTTPBlocklist.Parse(""))
	resetSharedServiceHTTP()
	t.Cleanup(func() {
		toggle.HTTPBlocklist.Reset()
		resetSharedServiceHTTP()
	})
}

func testExecutor() *executor {
	return NewExecutor(logr.Discard(), "test", &mockClient{}, apiConfig)
}

func getServiceCall(url string) *kyvernov1.APICall {
	return &kyvernov1.APICall{Method: "GET", Service: &kyvernov1.ServiceCall{URL: url}}
}

func Test_ExecuteServiceCall_BlocksLoopbackByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	var gotHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testExecutor().Execute(context.Background(), getServiceCall(srv.URL))
	assert.ErrorContains(t, err, "blocked")
	assert.Assert(t, !gotHit)
}

func Test_NewEgressPolicy_BareIPIsHostname(t *testing.T) {
	p, err := newEgressPolicy([]string{"169.254.169.254", "169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	assert.Equal(t, len(p.blockedCIDRs), 1)
	assert.Equal(t, len(p.blockedHosts), 1)
}

func Test_ExecuteServiceCall_BlocksMetadataHostByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	_, err := testExecutor().Execute(context.Background(), getServiceCall("http://169.254.169.254/latest/meta-data/"))
	assert.ErrorContains(t, err, "blocked")
}

func Test_ExecuteServiceCall_AllowlistRejectsOtherHosts(t *testing.T) {
	withEmptyEgressBlocklist(t)
	assert.NilError(t, toggle.HTTPAllowlist.Parse("https://api.example.com"))
	t.Cleanup(func() {
		toggle.HTTPAllowlist.Reset()
		resetSharedServiceHTTP()
	})

	_, err := testExecutor().Execute(context.Background(), getServiceCall("https://not-allowed.example.org/data"))
	assert.ErrorContains(t, err, "not permitted")
}

func Test_ExecuteServiceCall_AllowlistRejectsPathTraversal(t *testing.T) {
	withEmptyEgressBlocklist(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	assert.NilError(t, toggle.HTTPAllowlist.Parse(srv.URL+"/v1/"))
	t.Cleanup(func() {
		toggle.HTTPAllowlist.Reset()
		resetSharedServiceHTTP()
	})

	_, err := testExecutor().Execute(context.Background(), getServiceCall(srv.URL+"/v1/../admin"))
	assert.ErrorContains(t, err, "not permitted")

	_, err = testExecutor().Execute(context.Background(), getServiceCall(srv.URL+"/v1/%2e%2e/admin"))
	assert.ErrorContains(t, err, "not permitted")
}

func Test_ExecuteServiceCall_AllowlistRejectsRedirectToOtherHost(t *testing.T) {
	withEmptyEgressBlocklist(t)

	var sinkHit bool
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sinkHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"leaked":true}`))
	}))
	defer sink.Close()

	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusFound)
	}))
	defer src.Close()

	assert.NilError(t, toggle.HTTPAllowlist.Parse(src.URL))
	t.Cleanup(func() {
		toggle.HTTPAllowlist.Reset()
		resetSharedServiceHTTP()
	})

	_, err := testExecutor().Execute(context.Background(), getServiceCall(src.URL))
	assert.ErrorContains(t, err, "not permitted")
	assert.Assert(t, !sinkHit)
}

func Test_ExecuteServiceCall_BlocksIPv4MappedMetadataByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	_, err := testExecutor().Execute(context.Background(), getServiceCall("http://[::ffff:169.254.169.254]/latest/meta-data/"))
	assert.ErrorContains(t, err, "blocked")
}

func Test_ProxyAwareDestinationCheck_BlocksProxiedRequestToBlockedCIDR(t *testing.T) {
	policy, err := newEgressPolicy([]string{"127.0.0.0/8", "::1/128", "169.254.0.0/16"}, nil)
	assert.NilError(t, err)
	proxyURL := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	withProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }

	proxyFn := proxyAwareDestinationCheck(policy, withProxy)

	req, err := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	assert.NilError(t, err)
	_, err = proxyFn(req)
	assert.ErrorContains(t, err, "blocked")

	req, err = http.NewRequest(http.MethodGet, "http://localhost:9999/", nil)
	assert.NilError(t, err)
	_, err = proxyFn(req)
	assert.ErrorContains(t, err, "blocked")
}

func Test_ProxyAwareDestinationCheck_AllowsProxiedRequestToPermittedHost(t *testing.T) {
	policy, err := newEgressPolicy([]string{"169.254.0.0/16"}, nil)
	assert.NilError(t, err)
	proxyURL := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	withProxy := func(*http.Request) (*url.URL, error) { return proxyURL, nil }

	req, err := http.NewRequest(http.MethodGet, "http://8.8.8.8/", nil)
	assert.NilError(t, err)
	got, err := proxyAwareDestinationCheck(policy, withProxy)(req)
	assert.NilError(t, err)
	assert.Equal(t, got, proxyURL)
}

func Test_ProxyAwareDestinationCheck_DirectRequestsUnaffected(t *testing.T) {
	policy, err := newEgressPolicy([]string{"127.0.0.0/8"}, nil)
	assert.NilError(t, err)
	noProxy := func(*http.Request) (*url.URL, error) { return nil, nil }

	req, err := http.NewRequest(http.MethodGet, "http://localhost:9999/", nil)
	assert.NilError(t, err)
	got, err := proxyAwareDestinationCheck(policy, noProxy)(req)
	assert.NilError(t, err)
	assert.Assert(t, got == nil)
}

func Test_ExecuteServiceCall_BlocklistRejectsRedirectToMetadataHost(t *testing.T) {
	withEmptyEgressBlocklist(t)
	assert.NilError(t, toggle.HTTPBlocklist.Parse("metadata.google.internal"))
	resetSharedServiceHTTP()
	t.Cleanup(func() {
		toggle.HTTPBlocklist.Reset()
		resetSharedServiceHTTP()
	})

	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://metadata.google.internal/latest/meta-data/", http.StatusFound)
	}))
	defer src.Close()

	_, err := testExecutor().Execute(context.Background(), getServiceCall(src.URL))
	assert.ErrorContains(t, err, "blocked")
}

func caBundlePEM(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	cert := srv.Certificate()
	assert.Assert(t, cert != nil)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
}

func getServiceCallWithCA(url, caBundle string) *kyvernov1.APICall {
	return &kyvernov1.APICall{Method: "GET", Service: &kyvernov1.ServiceCall{URL: url, CABundle: caBundle}}
}

func Test_ExecuteServiceCall_CABundle_Success(t *testing.T) {
	withEmptyEgressBlocklist(t)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	got, err := testExecutor().Execute(context.Background(), getServiceCallWithCA(srv.URL, caBundlePEM(t, srv)))
	assert.NilError(t, err)
	assert.Assert(t, got != nil)
}

func Test_ExecuteServiceCall_CABundle_BlocksLoopbackByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	var gotHit bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := testExecutor().Execute(context.Background(), getServiceCallWithCA(srv.URL, caBundlePEM(t, srv)))
	assert.ErrorContains(t, err, "blocked")
	assert.Assert(t, !gotHit)
}

func Test_ExecuteServiceCall_CABundle_AllowlistRejectsRedirectToOtherHost(t *testing.T) {
	withEmptyEgressBlocklist(t)

	var sinkHit bool
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sinkHit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer sink.Close()

	src := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusFound)
	}))
	defer src.Close()

	assert.NilError(t, toggle.HTTPAllowlist.Parse(src.URL))
	t.Cleanup(func() {
		toggle.HTTPAllowlist.Reset()
		resetSharedServiceHTTP()
	})

	_, err := testExecutor().Execute(context.Background(), getServiceCallWithCA(src.URL, caBundlePEM(t, src)))
	assert.ErrorContains(t, err, "not permitted")
	assert.Assert(t, !sinkHit)
}

// withStubResolver replaces the resolver seam for the duration of a test. Resolution of
// a real name is not deterministic across platforms, and the unresolvable branch has to
// be exercised without depending on the CI environment's DNS.
func withStubResolver(t *testing.T, fn func(ctx context.Context, host string) ([]string, error)) {
	t.Helper()
	original := lookupHost
	lookupHost = fn
	t.Cleanup(func() { lookupHost = original })
}

func resolvesTo(ips ...string) func(context.Context, string) ([]string, error) {
	return func(context.Context, string) ([]string, error) { return ips, nil }
}

func resolutionFails() func(context.Context, string) ([]string, error) {
	return func(_ context.Context, host string) ([]string, error) {
		return nil, &net.DNSError{Err: "server misbehaving", Name: host}
	}
}

func Test_ProxyAwareDestinationCheck_RejectsHostnameResolvingIntoBlockedRange(t *testing.T) {
	withStubResolver(t, resolvesTo("169.254.169.254"))
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://internal.example.com/latest", nil)
	got, err := check(req)
	assert.ErrorContains(t, err, "blocked range")
	assert.Assert(t, got == nil)
}

func Test_ProxyAwareDestinationCheck_AllowsHostnameResolvingOutsideBlockedRanges(t *testing.T) {
	withStubResolver(t, resolvesTo("93.184.216.34"))
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://api.example.com/v1", nil)
	got, err := check(req)
	assert.NilError(t, err)
	assert.Equal(t, got, proxy)
}

// An unresolvable destination is a policy decision, not a config error: without an
// allowlist there is no way to check where the proxy will actually connect, so the
// request fails closed.
func Test_ProxyAwareDestinationCheck_RejectsUnresolvableHostnameWithoutAllowlist(t *testing.T) {
	withStubResolver(t, resolutionFails())
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://internal.example.com/latest", nil)
	got, err := check(req)
	assert.ErrorContains(t, err, "cannot be resolved in-pod")
	assert.ErrorContains(t, err, "--httpAllowlist")
	assert.Assert(t, got == nil)
}

// Proxied egress commonly cannot resolve external names in-pod. An allowlist is the
// operator's opt-in for those destinations: validateURL has already matched the URL
// against it before the request reaches the proxy function.
func Test_ProxyAwareDestinationCheck_AllowsUnresolvableHostnameWithAllowlist(t *testing.T) {
	withStubResolver(t, resolutionFails())
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, []string{"http://internal.example.com"})
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://internal.example.com/latest", nil)
	got, err := check(req)
	assert.NilError(t, err)
	assert.Equal(t, got, proxy)
}

// An allowlist must not excuse a destination that resolves into a blocked range; only
// the unresolvable case is opted back in.
func Test_ProxyAwareDestinationCheck_AllowlistDoesNotExcuseBlockedRange(t *testing.T) {
	withStubResolver(t, resolvesTo("169.254.169.254"))
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, []string{"http://internal.example.com"})
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://internal.example.com/latest", nil)
	got, err := check(req)
	assert.ErrorContains(t, err, "blocked range")
	assert.Assert(t, got == nil)
}

// Every IPv6 spelling of an IPv4 metadata address must be caught by the shipped default
// blocklist. Only ::ffff: is normalized by net.IPNet.Contains itself.
func Test_ExecuteServiceCall_BlocksEmbeddedIPv4EncodingsByDefault(t *testing.T) {
	for _, tc := range []struct {
		host string
		desc string
	}{
		{"[::ffff:169.254.169.254]", "IPv4-mapped"},
		{"[::169.254.169.254]", "IPv4-compatible"},
		{"[64:ff9b::169.254.169.254]", "NAT64 well-known prefix"},
		{"[2002:a9fe:a9fe::]", "6to4 of 169.254.169.254"},
		{"[2002:7f00:1::]", "6to4 of 127.0.0.1"},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			toggle.HTTPBlocklist.Reset()
			resetSharedServiceHTTP()

			_, err := testExecutor().Execute(context.Background(), getServiceCall("http://"+tc.host+"/latest/meta-data/"))
			assert.ErrorContains(t, err, "blocked")
		})
	}
}

// The AWS IMDS IPv6 endpoint needs no encoding trick, so it has to be on the default
// blocklist in its own right.
func Test_ExecuteServiceCall_BlocksIMDSIPv6EndpointByDefault(t *testing.T) {
	toggle.HTTPBlocklist.Reset()
	resetSharedServiceHTTP()

	_, err := testExecutor().Execute(context.Background(), getServiceCall("http://[fd00:ec2::254]/latest/meta-data/"))
	assert.ErrorContains(t, err, "blocked")
}

func Test_EmbeddedIPv4(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"::169.254.169.254", "169.254.169.254"},
		{"64:ff9b::169.254.169.254", "169.254.169.254"},
		{"2002:a9fe:a9fe::", "169.254.169.254"},
		{"2002:7f00:1::", "127.0.0.1"},
		// To4 already handles these, so there is nothing to extract.
		{"::ffff:169.254.169.254", ""},
		{"169.254.169.254", ""},
		// A plain IPv6 address must not be read as carrying an IPv4 address.
		{"2001:db8::1", ""},
		{"fd00:ec2::254", ""},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got := embeddedIPv4(net.ParseIP(tc.in))
			if tc.want == "" {
				assert.Assert(t, got == nil)
				return
			}
			assert.Equal(t, got.String(), tc.want)
		})
	}
}

// A denial must not report how the name resolved: that message reaches the caller through
// the admission response, PolicyReports and Events, and would otherwise act as a DNS
// resolution oracle for the controller's vantage point.
func Test_ValidateHostCIDRs_BlockedErrorDoesNotNameResolvedAddress(t *testing.T) {
	withStubResolver(t, resolvesTo("127.0.0.1"))
	policy, err := newEgressPolicy([]string{"127.0.0.0/8"}, nil)
	assert.NilError(t, err)

	err = policy.validateHostCIDRs(context.Background(), "internal.example.com")
	assert.ErrorContains(t, err, "blocked range 127.0.0.0/8")
	assert.Assert(t, !strings.Contains(err.Error(), "127.0.0.1"), "error must not reveal the resolved address: %v", err)
}

func Test_DialContext_BlockedErrorDoesNotNameResolvedAddress(t *testing.T) {
	withStubResolver(t, resolvesTo("169.254.169.254"))
	policy, err := newEgressPolicy([]string{"169.254.0.0/16"}, nil)
	assert.NilError(t, err)

	_, err = policy.dialContext()(context.Background(), "tcp", "internal.example.com:80")
	assert.ErrorContains(t, err, "blocked range 169.254.0.0/16")
	assert.Assert(t, !strings.Contains(err.Error(), "169.254.169.254"), "error must not reveal the resolved address: %v", err)
}

// A literal address supplied by the caller is not an oracle, so it stays in the message
// where it helps an operator.
func Test_ValidateHostCIDRs_LiteralAddressStaysInError(t *testing.T) {
	policy, err := newEgressPolicy([]string{"169.254.0.0/16"}, nil)
	assert.NilError(t, err)

	err = policy.validateHostCIDRs(context.Background(), "169.254.169.254")
	assert.ErrorContains(t, err, "169.254.169.254")
	assert.ErrorContains(t, err, "blocked range 169.254.0.0/16")
}

func Test_DialContext_BlocksHostnameResolvingToEmbeddedIPv4(t *testing.T) {
	withStubResolver(t, resolvesTo("64:ff9b::a9fe:a9fe"))
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)

	_, err = policy.dialContext()(context.Background(), "tcp", "rebind.example.com:80")
	assert.ErrorContains(t, err, "blocked range 169.254.169.254/32")
}

func Test_ProxyAwareDestinationCheck_RejectsBlockedIPDestination(t *testing.T) {
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://169.254.169.254/latest", nil)
	got, err := check(req)
	assert.ErrorContains(t, err, "blocked")
	assert.Assert(t, got == nil)
}

func Test_ProxyAwareDestinationCheck_AllowsPermittedIPDestination(t *testing.T) {
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	proxy := &url.URL{Scheme: "http", Host: "proxy.corp:3128"}
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return proxy, nil })

	req := httptest.NewRequest(http.MethodGet, "http://10.0.0.10/api", nil)
	got, err := check(req)
	assert.NilError(t, err)
	assert.Equal(t, got, proxy)
}

func Test_ProxyAwareDestinationCheck_NoProxyPassesThrough(t *testing.T) {
	policy, err := newEgressPolicy([]string{"169.254.169.254/32"}, nil)
	assert.NilError(t, err)
	check := proxyAwareDestinationCheck(policy, func(*http.Request) (*url.URL, error) { return nil, nil })

	req := httptest.NewRequest(http.MethodGet, "http://internal.example.com/latest", nil)
	got, err := check(req)
	assert.NilError(t, err)
	assert.Assert(t, got == nil)
}
