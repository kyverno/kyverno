package engine

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockPolicySelector mocks the PolicySelector interface for testing purposes.
type MockPolicySelector struct {
	exceptions []*kyvernov2.PolicyException
	err        error
}

// Find mocks finding exceptions based on the policy name and rule.
func (m *MockPolicySelector) Find(policyName, rule string) ([]*kyvernov2.PolicyException, error) {
	return m.exceptions, m.err
}

// createMockPolicy creates a mock policy with the given name for testing.
func createMockPolicy(name string) kyvernov1.PolicyInterface {
	return &kyvernov1.ClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v1",
			Kind:       "ClusterPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// createMockPolicyException creates a mock policy exception with the given name for testing.
func createMockPolicyException(name string) *kyvernov2.PolicyException {
	return &kyvernov2.PolicyException{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "kyverno.io/v2",
			Kind:       "PolicyException",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// TestGetPolicyExceptions_NoSelector tests the case when no exception selector is set.
func TestGetPolicyExceptions_NoSelector(t *testing.T) {
	engine := &engine{
		exceptionSelector: nil,
	}

	policy := createMockPolicy("test-policy")
	rule := "test-rule"

	exceptions, err := engine.GetPolicyExceptions(policy, rule)
	assert.NoError(t, err)
	assert.Nil(t, exceptions)
}

// TestGetPolicyExceptions_WithMatchingExceptions tests when matching exceptions are found.
func TestGetPolicyExceptions_WithMatchingExceptions(t *testing.T) {
	mockExceptions := []*kyvernov2.PolicyException{
		createMockPolicyException("exception-1"),
		createMockPolicyException("exception-2"),
	}

	engine := &engine{
		exceptionSelector: &MockPolicySelector{
			exceptions: mockExceptions,
			err:        nil,
		},
	}

	policy := createMockPolicy("test-policy")
	rule := "test-rule"

	exceptions, err := engine.GetPolicyExceptions(policy, rule)
	assert.NoError(t, err)
	assert.Equal(t, mockExceptions, exceptions)
}

// TestGetPolicyExceptions_WithNoMatchingExceptions tests when no matching exceptions are found.
func TestGetPolicyExceptions_WithNoMatchingExceptions(t *testing.T) {
	engine := &engine{
		exceptionSelector: &MockPolicySelector{
			exceptions: []*kyvernov2.PolicyException{},
			err:        nil,
		},
	}

	policy := createMockPolicy("test-policy")
	rule := "test-rule"

	exceptions, err := engine.GetPolicyExceptions(policy, rule)
	assert.NoError(t, err)
	assert.Empty(t, exceptions)
}

// TestGetPolicyExceptions_WithError tests when an error occurs while fetching exceptions.
func TestGetPolicyExceptions_WithError(t *testing.T) {
	mockError := errors.New("failed to fetch exceptions")

	engine := &engine{
		exceptionSelector: &MockPolicySelector{
			exceptions: nil,
			err:        mockError,
		},
	}

	policy := createMockPolicy("test-policy")
	rule := "test-rule"

	exceptions, err := engine.GetPolicyExceptions(policy, rule)
	assert.Error(t, err)
	assert.Nil(t, exceptions)
	assert.Equal(t, mockError, err)
}
