package report

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewAdmissionReport_NoOwnerWhenUIDEmpty(t *testing.T) {
	t.Parallel()

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deploy",
				"namespace": "default",
			},
		},
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	report := NewAdmissionReport("default", "admission-uid", gvr, gvk, resource)

	assert.Empty(t, report.GetOwnerReferences(), "ownerReferences should not be set when resource UID is empty")
	assert.Empty(t, GetResourceUid(report), "resource UID label should be empty")
}

func TestNewAdmissionReport_SetsOwnerWhenUIDPresent(t *testing.T) {
	t.Parallel()

	resource := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test-deploy",
				"namespace": "default",
				"uid":       "796f4672-c7ac-46e2-b630-94bbd516daed",
			},
		},
	}

	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	report := NewAdmissionReport("default", "admission-uid", gvr, gvk, resource)

	ownerRefs := report.GetOwnerReferences()
	assert.Len(t, ownerRefs, 1)
	assert.Equal(t, "apps/v1", ownerRefs[0].APIVersion)
	assert.Equal(t, "Deployment", ownerRefs[0].Kind)
	assert.Equal(t, "test-deploy", ownerRefs[0].Name)
	assert.Equal(t, types.UID("796f4672-c7ac-46e2-b630-94bbd516daed"), ownerRefs[0].UID)
	assert.Equal(t, types.UID("796f4672-c7ac-46e2-b630-94bbd516daed"), GetResourceUid(report))
}
