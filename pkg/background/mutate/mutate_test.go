package mutate

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// mockStatusControl implements StatusControlInterface for testing
type mockStatusControl struct {
	failedCalled  bool
	successCalled bool
	failedName    string
	successName   string
	failedMsg     string
	returnError   error
}

func (m *mockStatusControl) Failed(name string, message string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	m.failedCalled = true
	m.failedName = name
	m.failedMsg = message
	return nil, m.returnError
}

func (m *mockStatusControl) Success(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	m.successCalled = true
	m.successName = name
	return nil, m.returnError
}

func (m *mockStatusControl) Skip(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	return nil, m.returnError
}

func TestUpdateURStatus_SuccessCase(t *testing.T) {
	mock := &mockStatusControl{}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	err := updateURStatus(mock, ur, nil)

	assert.NoError(t, err)
	assert.True(t, mock.successCalled, "Success should be called when err is nil")
	assert.False(t, mock.failedCalled, "Failed should not be called when err is nil")
	assert.Equal(t, "test-ur", mock.successName)
}

func TestUpdateURStatus_FailureCase(t *testing.T) {
	mock := &mockStatusControl{}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}
	testErr := errors.New("mutation failed")

	err := updateURStatus(mock, ur, testErr)

	assert.NoError(t, err)
	assert.True(t, mock.failedCalled, "Failed should be called when err is not nil")
	assert.False(t, mock.successCalled, "Success should not be called when err is not nil")
	assert.Equal(t, "test-ur", mock.failedName)
	assert.Equal(t, "mutation failed", mock.failedMsg)
}

func TestUpdateURStatus_SuccessReturnsError(t *testing.T) {
	mock := &mockStatusControl{
		returnError: errors.New("status update failed"),
	}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}

	err := updateURStatus(mock, ur, nil)

	assert.Error(t, err)
	assert.Equal(t, "status update failed", err.Error())
	assert.True(t, mock.successCalled)
}

func TestUpdateURStatus_FailedReturnsError(t *testing.T) {
	mock := &mockStatusControl{
		returnError: errors.New("status update failed"),
	}
	ur := kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ur",
		},
	}
	testErr := errors.New("mutation failed")

	err := updateURStatus(mock, ur, testErr)

	assert.Error(t, err)
	assert.Equal(t, "status update failed", err.Error())
	assert.True(t, mock.failedCalled)
}

func TestUpdateURStatus_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name            string
		urName          string
		inputErr        error
		statusErr       error
		expectSuccess   bool
		expectFailed    bool
		expectedErr     bool
		expectedErrMsg  string
		expectedFailMsg string
	}{
		{
			name:          "success with no errors",
			urName:        "ur-1",
			inputErr:      nil,
			statusErr:     nil,
			expectSuccess: true,
			expectFailed:  false,
			expectedErr:   false,
		},
		{
			name:            "failure with error message",
			urName:          "ur-2",
			inputErr:        errors.New("resource not found"),
			statusErr:       nil,
			expectSuccess:   false,
			expectFailed:    true,
			expectedErr:     false,
			expectedFailMsg: "resource not found",
		},
		{
			name:           "success but status update fails",
			urName:         "ur-3",
			inputErr:       nil,
			statusErr:      errors.New("api server unavailable"),
			expectSuccess:  true,
			expectFailed:   false,
			expectedErr:    true,
			expectedErrMsg: "api server unavailable",
		},
		{
			name:            "failure and status update fails",
			urName:          "ur-4",
			inputErr:        errors.New("validation error"),
			statusErr:       errors.New("network timeout"),
			expectSuccess:   false,
			expectFailed:    true,
			expectedErr:     true,
			expectedErrMsg:  "network timeout",
			expectedFailMsg: "validation error",
		},
		{
			name:          "empty error message",
			urName:        "ur-5",
			inputErr:      errors.New(""),
			statusErr:     nil,
			expectSuccess: false,
			expectFailed:  true,
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStatusControl{
				returnError: tt.statusErr,
			}
			ur := kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.urName,
				},
			}

			err := updateURStatus(mock, ur, tt.inputErr)

			if tt.expectedErr {
				assert.Error(t, err)
				if tt.expectedErrMsg != "" {
					assert.Equal(t, tt.expectedErrMsg, err.Error())
				}
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectSuccess, mock.successCalled, "Success call mismatch")
			assert.Equal(t, tt.expectFailed, mock.failedCalled, "Failed call mismatch")

			if tt.expectSuccess {
				assert.Equal(t, tt.urName, mock.successName)
			}
			if tt.expectFailed {
				assert.Equal(t, tt.urName, mock.failedName)
				if tt.expectedFailMsg != "" {
					assert.Equal(t, tt.expectedFailMsg, mock.failedMsg)
				}
			}
		})
	}
}

func TestUpdateURStatus_WithDifferentURNames(t *testing.T) {
	tests := []struct {
		name   string
		urName string
	}{
		{
			name:   "simple name",
			urName: "simple-ur",
		},
		{
			name:   "namespaced format",
			urName: "namespace/policy-name",
		},
		{
			name:   "long name",
			urName: "very-long-update-request-name-with-many-segments",
		},
		{
			name:   "name with numbers",
			urName: "ur-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStatusControl{}
			ur := kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: tt.urName,
				},
			}

			err := updateURStatus(mock, ur, nil)

			assert.NoError(t, err)
			assert.Equal(t, tt.urName, mock.successName)
		})
	}
}

func TestUpdateURStatus_WithVariousErrorTypes(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{
			name:     "standard error",
			inputErr: errors.New("standard error message"),
		},
		{
			name:     "empty patch error",
			inputErr: ErrEmptyPatch,
		},
		{
			name:     "wrapped error",
			inputErr: errors.New("wrapped: original error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockStatusControl{}
			ur := kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ur",
				},
			}

			err := updateURStatus(mock, ur, tt.inputErr)

			assert.NoError(t, err)
			assert.True(t, mock.failedCalled)
			assert.Equal(t, tt.inputErr.Error(), mock.failedMsg)
		})
	}
}
