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
//   - UPDATE: object is the resource, oldObject is oldResource if provided, else resource
//   - DELETE: object is null, oldObject is oldResource if provided, else resource
func AdmissionRequestShape(operation string, resource *unstructured.Unstructured, oldResource ...*unstructured.Unstructured) (admissionv1.Operation, runtime.Object, runtime.Object) {
	switch operation {
	case "UPDATE":
		var old runtime.Object
		if len(oldResource) > 0 && oldResource[0] != nil {
			old = oldResource[0].DeepCopy()
		} else if resource != nil {
			old = resource.DeepCopy()
		}
		return admissionv1.Update, resource, old
	case "DELETE":
		// deep copy so downstream mutations of the old state cannot leak into
		// the shared resource object
		var old runtime.Object
		if len(oldResource) > 0 && oldResource[0] != nil {
			old = oldResource[0].DeepCopy()
		} else if resource != nil {
			old = resource.DeepCopy()
		}
		return admissionv1.Delete, nil, old
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


func (p *PolicyProcessor) resolveOldObject(resource *unstructured.Unstructured, policyName ...string) *unstructured.Unstructured {
	if p.Variables == nil {
		return nil
	}
	var resName string
	if resource != nil {
		resName = resource.GetName()
	}
	return p.Variables.OldObject(resource, resName, policyName...)
}
