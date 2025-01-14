package policy

import (
	"github.com/google/cel-go/cel"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

type CompiledPolicy struct {
	failurePolicy    admissionregistrationv1.FailurePolicyType
	matchConditions  []cel.Program
	variables        map[string]cel.Program
	validations      []cel.Program
	auditAnnotations map[string]cel.Program
}
