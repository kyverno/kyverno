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

// NormalizeValuesOperation validates a `request.operation` global value from a
// values file. In addition to the operations supported by admission request
// simulation, CONNECT is accepted for backward compatibility with classic
// policies that evaluate `request.operation` in preconditions; it is not
// simulated for CEL policy types, which fall back to the CREATE request shape.
func NormalizeValuesOperation(operation string) (string, error) {
	switch operation {
	case "", "CREATE", "UPDATE", "DELETE", "CONNECT":
		return operation, nil
	default:
		return "", fmt.Errorf("invalid request.operation value %q, must be one of CREATE, UPDATE, DELETE, CONNECT", operation)
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
		// deep copy so downstream mutations of the old state cannot leak into
		// the shared resource object
		return admissionv1.Delete, nil, resource.DeepCopy()
	default:
		return admissionv1.Create, resource, nil
	}
}

// resolveOperation returns the effective operation for the processor: the
// explicitly configured operation takes precedence, then the `request.operation`
// global value from the values file, then the default (CREATE). CONNECT from the
// values file is not simulated and maps to the default request shape.
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
