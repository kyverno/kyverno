package apicall

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
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

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
