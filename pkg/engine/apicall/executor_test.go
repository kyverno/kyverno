package apicall

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
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
