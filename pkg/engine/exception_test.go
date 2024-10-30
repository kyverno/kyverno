package engine

import (
	"errors"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// Custom mock for exceptionSelector
type mockExceptionSelector struct {
	findFunc func(policyName, rule string) ([]*kyvernov2.PolicyException, error)
}

func (m *mockExceptionSelector) Find(policyName, rule string) ([]*kyvernov2.PolicyException, error) {
	return m.findFunc(policyName, rule)
}

func TestGetPolicyExceptions(t *testing.T) {
	// Mock data
	mockPolicyName := "test-policy"
	mockRule := "test-rule"
	mockPolicy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: mockPolicyName,
		},
	}

	// Define test cases
	tests := []struct {
		name               string
		exceptionSelector  *mockExceptionSelector
		expectedExceptions []*kyvernov2.PolicyException
		expectedError      error
	}{
		{
			name:               "No exception selector",
			exceptionSelector:  nil,
			expectedExceptions: nil,
			expectedError:      nil,
		},
		{
			name: "Exception selector finds matching exception",
			exceptionSelector: &mockExceptionSelector{
				findFunc: func(policyName, rule string) ([]*kyvernov2.PolicyException, error) {
					if policyName == cache.MetaObjectToName(mockPolicy).String() && rule == mockRule {
						return []*kyvernov2.PolicyException{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name: "exception1",
								},
							},
						}, nil
					}
					return nil, nil
				},
			},
			expectedExceptions: []*kyvernov2.PolicyException{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "exception1",
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Exception selector finds no matching exception",
			exceptionSelector: &mockExceptionSelector{
				findFunc: func(policyName, rule string) ([]*kyvernov2.PolicyException, error) {
					return nil, nil
				},
			},
			expectedExceptions: nil,
			expectedError:      nil,
		},
		{
			name: "Exception selector returns error",
			exceptionSelector: &mockExceptionSelector{
				findFunc: func(policyName, rule string) ([]*kyvernov2.PolicyException, error) {
					return nil, errors.New("some error")
				},
			},
			expectedExceptions: nil,
			expectedError:      errors.New("some error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize engine with mock exceptionSelector
			e := &engine{
				exceptionSelector: tt.exceptionSelector,
			}
			// Call GetPolicyExceptions
			exceptions, err := e.GetPolicyExceptions(mockPolicy, mockRule)
			// Assert results
			assert.Equal(t, tt.expectedExceptions, exceptions)
			assert.Equal(t, tt.expectedError, err)
		})
	}
}
