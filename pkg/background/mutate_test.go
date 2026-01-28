package background

import (
	"context"
	"testing"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestHandleMutatePolicyAbsence_Success(t *testing.T) {
	// Create a fake Kyverno client
	kyvernoClient := fake.NewSimpleClientset()

	// Create controller with fake client
	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	// Create an UpdateRequest
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "test-policy",
		},
	}

	// Call the method
	err := c.handleMutatePolicyAbsence(ur)

	// Verify no error
	assert.NoError(t, err, "handleMutatePolicyAbsence should succeed")

	// Verify DeleteCollection was called
	actions := kyvernoClient.Actions()
	assert.Len(t, actions, 1, "should have one action")
	assert.Equal(t, "delete-collection", actions[0].GetVerb(), "should call DeleteCollection")
}

func TestHandleMutatePolicyAbsence_WithNamespacedPolicy(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-namespaced",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "default/namespaced-policy",
		},
	}

	err := c.handleMutatePolicyAbsence(ur)

	assert.NoError(t, err)
	actions := kyvernoClient.Actions()
	assert.Len(t, actions, 1)
	assert.Equal(t, "delete-collection", actions[0].GetVerb())
}

func TestHandleMutatePolicyAbsence_NilUpdateRequest(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	// Call with nil UpdateRequest - should panic or handle gracefully
	// In Go, dereferencing nil will panic, so we expect a panic here
	assert.Panics(t, func() {
		_ = c.handleMutatePolicyAbsence(nil)
	}, "should panic when UpdateRequest is nil")
}

func TestHandleMutatePolicyAbsence_EmptyPolicyName(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-empty-policy",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "",
		},
	}

	err := c.handleMutatePolicyAbsence(ur)

	// Should succeed even with empty policy name (label selector will be empty)
	assert.NoError(t, err)
	actions := kyvernoClient.Actions()
	assert.Len(t, actions, 1)
}

func TestHandleMutatePolicyAbsence_DeleteCollectionError(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	// Add a reactor to simulate deletion error
	kyvernoClient.PrependReactor("delete-collection", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, assert.AnError
	})

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-error",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "error-policy",
		},
	}

	err := c.handleMutatePolicyAbsence(ur)

	// Should return the error from DeleteCollection
	assert.Error(t, err, "should return error from DeleteCollection")
	assert.Equal(t, assert.AnError, err)
}

func TestHandleMutatePolicyAbsence_VerifyLabelSelector(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	// Track the label selector used
	var capturedLabelSelector string
	kyvernoClient.PrependReactor("delete-collection", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		deleteCollectionAction := action.(k8stesting.DeleteCollectionAction)
		capturedLabelSelector = deleteCollectionAction.GetListRestrictions().Labels.String()
		return true, nil, nil
	})

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-labels",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "my-policy",
		},
	}

	err := c.handleMutatePolicyAbsence(ur)

	assert.NoError(t, err)
	// Verify label selector contains the policy name
	assert.Contains(t, capturedLabelSelector, "my-policy", "label selector should contain policy name")
}

func TestHandleMutatePolicyAbsence_ContextCancellation(t *testing.T) {
	kyvernoClient := fake.NewSimpleClientset()

	// Simulate context cancellation
	kyvernoClient.PrependReactor("delete-collection", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, context.Canceled
	})

	c := &controller{
		kyvernoClient: kyvernoClient,
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-context",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "context-policy",
		},
	}

	err := c.handleMutatePolicyAbsence(ur)

	// Should return context.Canceled error
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestHandleMutatePolicyAbsence_MultipleScenarios(t *testing.T) {
	tests := []struct {
		name          string
		policyName    string
		simulateError bool
		expectError   bool
	}{
		{
			name:          "valid cluster policy",
			policyName:    "cluster-policy",
			simulateError: false,
			expectError:   false,
		},
		{
			name:          "valid namespaced policy",
			policyName:    "namespace/policy-name",
			simulateError: false,
			expectError:   false,
		},
		{
			name:          "policy with special characters",
			policyName:    "policy-with-dashes-123",
			simulateError: false,
			expectError:   false,
		},
		{
			name:          "deletion fails",
			policyName:    "failing-policy",
			simulateError: true,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kyvernoClient := fake.NewSimpleClientset()

			if tt.simulateError {
				kyvernoClient.PrependReactor("delete-collection", "updaterequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, assert.AnError
				})
			}

			c := &controller{
				kyvernoClient: kyvernoClient,
			}

			ur := &kyvernov2.UpdateRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-ur-" + tt.name,
					Namespace: "kyverno",
				},
				Spec: kyvernov2.UpdateRequestSpec{
					Policy: tt.policyName,
				},
			}

			err := c.handleMutatePolicyAbsence(ur)

			if tt.expectError {
				assert.Error(t, err, "expected error for test case: %s", tt.name)
			} else {
				assert.NoError(t, err, "expected no error for test case: %s", tt.name)
			}

			// Verify DeleteCollection was called
			actions := kyvernoClient.Actions()
			assert.Len(t, actions, 1, "should have one action for test case: %s", tt.name)
			assert.Equal(t, "delete-collection", actions[0].GetVerb())
		})
	}
}
