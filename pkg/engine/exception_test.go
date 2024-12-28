package engine

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Mocking the exceptionSelector
type MockExceptionSelector struct {
	mock.Mock
}

func (m *MockExceptionSelector) Find(policyName string, rule string) ([]*kyvernov2.PolicyException, error) {
	args := m.Called(policyName, rule)
	return args.Get(0).([]*kyvernov2.PolicyException), args.Error(1)
}

func TestGetPolicyExceptions(t *testing.T) {
	tests := []struct {
		name               string
		policy             *kyvernov1.Policy
		rule               string
		mockReturnValue    []*kyvernov2.PolicyException
		mockReturnError    error
		expectedExceptions []*kyvernov2.PolicyException
		expectedError      error
	}{
		{
			name: "Valid exceptions returned",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy",
				},
			},
			rule: "test-rule",
			mockReturnValue: []*kyvernov2.PolicyException{
				{ /* Populate with necessary data */ },
				{ /* Another exception */ },
			},
			mockReturnError: nil,
			expectedExceptions: []*kyvernov2.PolicyException{
				{ /* Expected exception data */ },
				{ /* Another expected exception */ },
			},
			expectedError: nil,
		},
		{
			name: "No exceptions returned",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-no-exceptions",
				},
			},
			rule:               "test-rule",
			mockReturnValue:    nil,
			mockReturnError:    nil,
			expectedExceptions: nil,
			expectedError:      nil,
		},
		{
			name: "Selector is nil",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-no-selector",
				},
			},
			rule:               "test-rule",
			mockReturnValue:    nil,
			mockReturnError:    nil,
			expectedExceptions: nil,
			expectedError:      nil,
		},
		{
			name: "Error in Find method",
			policy: &kyvernov1.Policy{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-policy-error",
				},
			},
			rule:               "test-rule",
			mockReturnValue:    nil,
			mockReturnError:    assert.AnError,
			expectedExceptions: nil,
			expectedError:      assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create an instance of MockExceptionSelector
			mockSelector := new(MockExceptionSelector)

			// Mock the Find method to return the specified mock values
			mockSelector.On("Find", tt.policy.Name, tt.rule).Return(tt.mockReturnValue, tt.mockReturnError)

			// Create the engine object with the mock selector
			engine := &engine{
				exceptionSelector: mockSelector,
			}

			// Call the GetPolicyExceptions method
			exceptions, err := engine.GetPolicyExceptions(tt.policy, tt.rule)

			// Assertions
			assert.ErrorIs(t, err, tt.expectedError)
			assert.ElementsMatch(t, tt.expectedExceptions, exceptions)

			mockSelector.AssertExpectations(t)
		})
	}
}
