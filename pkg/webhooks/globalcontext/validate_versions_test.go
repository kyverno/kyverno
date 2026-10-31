package globalcontext

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
)

func TestGlobalContextValidationAcrossVersions(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"v2", "v2beta1", "v2alpha1"} {
		for _, operation := range []admissionv1.Operation{admissionv1.Create, admissionv1.Update} {
			for _, valid := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s/%s/valid=%t", version, operation, valid), func(t *testing.T) {
					t.Parallel()
					query := "[].metadata.name"
					if !valid {
						query = "["
					}
					object := map[string]interface{}{
						"apiVersion": "kyverno.io/" + version,
						"kind":       "GlobalContextEntry",
						"metadata":   map[string]interface{}{"name": "cache"},
						"spec": map[string]interface{}{
							"kubernetesResource": map[string]interface{}{"version": "v1", "resource": "configmaps"},
							"projections":        []interface{}{map[string]interface{}{"name": "names", "jmesPath": query}},
						},
					}
					request := admissionRequestWithObject(t, object)
					request.Operation = operation
					if operation == admissionv1.Update {
						request.OldObject = request.Object
					}
					response := NewHandlers().Validate(context.Background(), logr.Discard(), request, "", time.Now())
					if response.Allowed != valid {
						t.Fatalf("validation allowed=%t, want %t: %#v", response.Allowed, valid, response.Result)
					}
				})
			}
		}
	}
}
