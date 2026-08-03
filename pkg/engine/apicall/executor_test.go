package apicall

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/toggle"
	"gotest.tools/assert"
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
	// Clear the blocklist so the loopback test server is reachable.
	assert.NilError(t, toggle.HTTPBlocklist.Parse(""))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

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

func Test_validateServiceURL_BlocksHostname(t *testing.T) {
	assert.NilError(t, toggle.HTTPBlocklist.Parse("metadata.google.internal,169.254.169.254"))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

	err := validateServiceURL("http://metadata.google.internal/computeMetadata/v1/")
	assert.ErrorContains(t, err, "blocked")
	assert.ErrorContains(t, err, "metadata.google.internal")
}

func Test_validateServiceURL_BlocksExactIP(t *testing.T) {
	assert.NilError(t, toggle.HTTPBlocklist.Parse("169.254.169.254"))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

	err := validateServiceURL("http://169.254.169.254/latest/meta-data/")
	assert.ErrorContains(t, err, "blocked")
}

func Test_validateServiceURL_AllowsUnblockedURL(t *testing.T) {
	assert.NilError(t, toggle.HTTPBlocklist.Parse("169.254.169.254"))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

	err := validateServiceURL("https://my-webhook.my-namespace.svc.cluster.local/validate")
	assert.NilError(t, err)
}

func Test_validateServiceURL_AllowlistPermitsMatch(t *testing.T) {
	assert.NilError(t, toggle.HTTPAllowlist.Parse("https://allowed.example.com"))
	t.Cleanup(func() { toggle.HTTPAllowlist.Reset() })

	err := validateServiceURL("https://allowed.example.com/path")
	assert.NilError(t, err)
}

func Test_validateServiceURL_AllowlistBlocksNonMatch(t *testing.T) {
	assert.NilError(t, toggle.HTTPAllowlist.Parse("https://allowed.example.com"))
	t.Cleanup(func() { toggle.HTTPAllowlist.Reset() })

	err := validateServiceURL("https://other.example.com/path")
	assert.ErrorContains(t, err, "not permitted")
}

func Test_validateServiceURL_SkipsCIDREntries(t *testing.T) {
	// CIDR entries in the blocklist are enforced at dial time, not at URL validation.
	assert.NilError(t, toggle.HTTPBlocklist.Parse("127.0.0.0/8"))
	t.Cleanup(func() { toggle.HTTPBlocklist.Reset() })

	// validateServiceURL itself should not error for CIDR-blocked addresses —
	// that check happens in secureDialContext when the connection is made.
	err := validateServiceURL("http://127.0.0.1/path")
	assert.NilError(t, err)
}

func Test_ExecuteServiceCall_BlocksLoopbackViaCIDR(t *testing.T) {
	// Default blocklist includes 127.0.0.0/8; ensure service calls to loopback are rejected.
	toggle.HTTPBlocklist.Reset()

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer s.Close()

	executor := NewExecutor(logr.Discard(), "test-call", &mockClient{}, apiConfig)
	call := &kyvernov1.APICall{
		Method: "GET",
		Service: &kyvernov1.ServiceCall{
			URL: s.URL, // 127.0.0.1:PORT — blocked by default 127.0.0.0/8
		},
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "blocked")
}

func Test_ExecuteServiceCall_BlocksMetadataHostname(t *testing.T) {
	// Default blocklist includes 169.254.169.254 hostname.
	toggle.HTTPBlocklist.Reset()

	executor := NewExecutor(logr.Discard(), "test-call", &mockClient{}, apiConfig)
	call := &kyvernov1.APICall{
		Method: "GET",
		Service: &kyvernov1.ServiceCall{
			URL: "http://169.254.169.254/latest/meta-data/",
		},
	}

	_, err := executor.Execute(context.TODO(), call)
	assert.ErrorContains(t, err, "blocked")
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
