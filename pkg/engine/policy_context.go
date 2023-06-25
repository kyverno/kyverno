package engine

import (
	"github.com/kyverno/kyverno/pkg/engine/policycontext"
)

// PolicyContext contains the contexts for engine to process
type PolicyContext = policycontext.PolicyContext

var (
	NewPolicyContext                     = policycontext.NewPolicyContext
	NewPolicyContextFromAdmissionRequest = policycontext.NewPolicyContextFromAdmissionRequest
)
