package apicall

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockClient implements ClientInterface for testing
type mockClient struct {
	response []byte
	err      error
}

func (m *mockClient) RawAbsPath(ctx context.Context, path string, method string, dataReader io.Reader) ([]byte, error) {
	return m.response, m.err
}

func TestExecuteK8sAPICall_ForbiddenError(t *testing.T) {
	tests := []struct {
		name          string
		clientErr     error
		expectContain string
	}{
		{
			name: "403 Forbidden - should return helpful RBAC message",
			clientErr: apierrors.NewForbidden(
				kyvernov1.Resource("csistoragecapacities"),
				"",
				errors.New("User \"system:serviceaccount:kyverno:kyverno-admission-controller\" cannot list resource \"csistoragecapacities\" in API group \"storage.k8s.io\" at the cluster scope"),
			),
			expectContain: "permission denied: Kyverno service account lacks RBAC permissions",
		},
		{
			name: "403 Forbidden with specific resource",
			clientErr: &apierrors.StatusError{
				ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Message: "volumeattachments.storage.k8s.io is forbidden",
					Reason:  metav1.StatusReasonForbidden,
					Code:    403,
				},
			},
			expectContain: "Grant the required permissions in a ClusterRole/Role",
		},
		{
			name:          "Other errors - should return generic message",
			clientErr:     errors.New("connection timeout"),
			expectContain: "failed to GET resource with raw url",
		},
		{
			name: "404 Not Found - should return generic message",
			clientErr: apierrors.NewNotFound(
				kyvernov1.Resource("pods"),
				"test-pod",
			),
			expectContain: "failed to GET resource with raw url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockClient{
				err: tt.clientErr,
			}

			executor := NewExecutor(
				logr.Discard(),
				"test-entry",
				client,
				APICallConfiguration{},
			)

			_, err := executor.executeK8sAPICall(
				context.Background(),
				"/apis/storage.k8s.io/v1/csistoragecapacities",
				"GET",
				nil,
			)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectContain)

			// For Forbidden errors, ensure it mentions RBAC
			if apierrors.IsForbidden(tt.clientErr) {
				assert.Contains(t, err.Error(), "RBAC")
				assert.Contains(t, err.Error(), "service account")
			}
		})
	}
}

func TestExecuteK8sAPICall_Success(t *testing.T) {
	expectedData := []byte(`{"items": []}`)
	client := &mockClient{
		response: expectedData,
		err:      nil,
	}

	executor := NewExecutor(
		logr.Discard(),
		"test-entry",
		client,
		APICallConfiguration{},
	)

	data, err := executor.executeK8sAPICall(
		context.Background(),
		"/api/v1/pods",
		"GET",
		nil,
	)

	assert.NoError(t, err)
	assert.Equal(t, expectedData, data)
}

func TestExecuteK8sAPICall_ForbiddenPreservesOriginalError(t *testing.T) {
	originalMsg := "User \"system:serviceaccount:kyverno:kyverno-admission-controller\" cannot list resource \"volumeattachments\" in API group \"storage.k8s.io\" at the cluster scope"
	
	client := &mockClient{
		err: &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status:  metav1.StatusFailure,
				Message: originalMsg,
				Reason:  metav1.StatusReasonForbidden,
				Code:    403,
			},
		},
	}

	executor := NewExecutor(
		logr.Discard(),
		"test-entry",
		client,
		APICallConfiguration{},
	)

	_, err := executor.executeK8sAPICall(
		context.Background(),
		"/apis/storage.k8s.io/v1/volumeattachments",
		"GET",
		nil,
	)

	assert.Error(t, err)
	
	// Should contain the helpful message
	assert.Contains(t, err.Error(), "permission denied")
	assert.Contains(t, err.Error(), "RBAC permissions")
	
	// Should also preserve the original error details
	errMsg := err.Error()
	assert.True(t, strings.Contains(errMsg, "volumeattachments") || strings.Contains(errMsg, "storage.k8s.io"),
		"error should preserve original resource/API group details")
}
