package background

import (
	"testing"

	kyvernov1beta1 "github.com/kyverno/kyverno/api/kyverno/v1beta1"
)

func TestHandleMutatePolicyAbsence(t *testing.T) {
	// Create a new controller.
	c := &controller{}

	// Create a new update request.
	ur := &kyvernov1beta1.UpdateRequest{
		Spec: kyvernov1beta1.UpdateRequestSpec{
			Policy: "test-policy",
		},
	}

	// Test that the `handleMutatePolicyAbsence` function succeeds when all of the parameters are valid.
	err := c.handleMutatePolicyAbsence(ur)
	if err != nil {
		t.Errorf("Expected the `handleMutatePolicyAbsence` function to succeed, but got an error: %v", err)
	}
}
