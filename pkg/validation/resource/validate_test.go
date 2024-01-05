package resource

import (
	"context"
	"testing"

	admissionutils "github.com/kyverno/kyverno/pkg/utils/admission"
	"gotest.tools/assert"
	v1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	ctx = context.Background()

	pod = `{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "test-pod",
			"namespace": "",
			"labels": {
				"cleanup.kyverno.io/ttl": "1d"
			}
		},
		"spec": {
			"containers": [
				{
				"name": "nginx",
				"image": "nginx:latest"
				}
			]
		}
	}
`

	admissionRequest = v1.AdmissionRequest{
		Operation: v1.Create,
		Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
		Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
		Object: runtime.RawExtension{
			Raw: []byte(pod),
		},
		RequestResource: &metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	}
)

func Test_ValidateTTL(t *testing.T) {
	metadata, _, err := admissionutils.GetPartialObjectMetadatas(admissionRequest)
	assert.NilError(t, err)

	err = ValidateTtlLabel(ctx, metadata)
	assert.NilError(t, err)
}
