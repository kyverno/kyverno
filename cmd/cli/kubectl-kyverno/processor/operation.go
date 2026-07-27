package processor

import (
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// NormalizeOperation validates a CLI-declared admission operation. An empty
// string is valid and means "use the default operation".
func NormalizeOperation(operation string) (string, error) {
	switch operation {
	case "", "CREATE", "UPDATE", "DELETE":
		return operation, nil
	default:
		return "", fmt.Errorf("invalid operation %q, must be one of CREATE, UPDATE, DELETE", operation)
	}
}

// AdmissionRequestShape maps a CLI-declared operation to the admission operation
// and the (object, oldObject) pair, mirroring the API server semantics:
//   - CREATE (default): object is the resource, oldObject is null
//   - UPDATE: object and oldObject are both the resource
//   - DELETE: object is null, oldObject is the resource
func AdmissionRequestShape(operation string, resource *unstructured.Unstructured) (admissionv1.Operation, runtime.Object, runtime.Object) {
	switch operation {
	case "UPDATE":
		return admissionv1.Update, resource, resource.DeepCopy()
	case "DELETE":
		return admissionv1.Delete, nil, resource
	default:
		return admissionv1.Create, resource, nil
	}
}

// resolveOperation returns the effective operation for the processor: the
// explicitly configured operation takes precedence, then the `request.operation`
// global value from the values file, then the default (CREATE).
func (p *PolicyProcessor) resolveOperation() string {
	if p.Operation != "" {
		return p.Operation
	}
	if p.Variables != nil {
		if op, err := NormalizeOperation(p.Variables.GlobalOperation()); err == nil {
			return op
		}
	}
	return ""
}
