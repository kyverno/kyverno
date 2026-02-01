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
// It tracks which methods were called and allows simulating errors
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
