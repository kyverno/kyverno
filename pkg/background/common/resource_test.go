package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

// fakeListClient wraps dclient.Interface and overrides ListResource and
// GetResource to return pre-configured namespaces. This pattern matches
// existing test helpers in pkg/background/generate/cleanup_test.go.
type fakeListClient struct {
	dclient.Interface
	namespaces []unstructured.Unstructured
}

func (f *fakeListClient) ListResource(_ context.Context, _, _, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{Items: f.namespaces}, nil
}

func (f *fakeListClient) GetResource(_ context.Context, _, _, _ string, name string, _ ...string) (*unstructured.Unstructured, error) {
	for i := range f.namespaces {
		if f.namespaces[i].GetName() == name {
			return &f.namespaces[i], nil
		}
	}
	return nil, fmt.Errorf("resource not found: %s", name)
}

func newFakeClientWithNamespace(name string, uid types.UID, labels map[string]string) dclient.Interface {
	ns := &unstructured.Unstructured{}
	ns.SetAPIVersion("v1")
	ns.SetKind("Namespace")
	ns.SetName(name)
	ns.SetUID(uid)
	ns.SetLabels(labels)
	return &fakeListClient{
		Interface:  dclient.NewEmptyFakeClient(),
		namespaces: []unstructured.Unstructured{*ns},
	}
}

func makeNamespaceSpec(name string, uid types.UID) kyvernov1.ResourceSpec {
	return kyvernov1.ResourceSpec{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       name,
		UID:        uid,
	}
}

func TestGetResource_UIDMatch(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "match-uid-111", map[string]string{"layer": "business"})
	spec := makeNamespaceSpec("test-ns", "match-uid-111")
	urSpec := kyvernov2.UpdateRequestSpec{}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-ns", result.GetName())
	assert.Equal(t, types.UID("match-uid-111"), result.GetUID())
	assert.Equal(t, "business", result.GetLabels()["layer"])
}

func TestGetResource_UIDMismatch_SameName(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", map[string]string{"layer": "operational"})
	spec := makeNamespaceSpec("test-ns", "phantom-uid-999")
	urSpec := kyvernov2.UpdateRequestSpec{}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.Error(t, err, "GetResource should return an error when UID does not match any live object")
	assert.ErrorContains(t, err, "phantom-uid-999")
	assert.Nil(t, result, "GetResource should return nil when UID does not match any live object")
}

func TestGetResource_UIDMismatch_DeletedRecreated(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "new-uid-555", nil)
	spec := makeNamespaceSpec("test-ns", "old-uid-123")
	urSpec := kyvernov2.UpdateRequestSpec{}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.Error(t, err, "GetResource should return an error when object was deleted and recreated with a new UID")
	assert.Nil(t, result, "GetResource should return nil when object was deleted and recreated with a new UID")
}

func TestGetResource_NoUID_NameLookup(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", nil)
	spec := kyvernov1.ResourceSpec{
		APIVersion: "v1",
		Kind:       "Namespace",
		Name:       "test-ns",
	}
	urSpec := kyvernov2.UpdateRequestSpec{}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-ns", result.GetName())
	assert.Equal(t, types.UID("live-uid-111"), result.GetUID())
}

func TestGetResource_UIDNotFound_AdmissionRequestFallback(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", nil)
	spec := makeNamespaceSpec("test-ns", "phantom-uid-999")
	admissionReq := &admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Object: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"test-ns","uid":"phantom-uid-999","labels":{"layer":"business"}}}`),
		},
	}
	urSpec := kyvernov2.UpdateRequestSpec{
		Context: kyvernov2.UpdateRequestSpecContext{
			AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
				AdmissionRequest: admissionReq,
			},
		},
	}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.Error(t, err, "GetResource should surface a descriptive error instead of falling back to admission request bytes")
	assert.Nil(t, result, "GetResource should NOT fall back to admission request bytes when UID lookup fails")
}

func TestGetResource_UIDNotFound_NoAdmissionRequest(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", nil)
	spec := makeNamespaceSpec("test-ns", "phantom-uid-999")
	urSpec := kyvernov2.UpdateRequestSpec{}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetResource_UIDMismatch_Subresource_ReturnsAdmissionObject(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", nil)
	spec := makeNamespaceSpec("test-ns", "phantom-uid-999")
	admissionReq := &admissionv1.AdmissionRequest{
		Operation:   admissionv1.Create,
		SubResource: "status",
		Object: runtime.RawExtension{
			Raw: []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"test-ns","uid":"phantom-uid-999","labels":{"layer":"business"}}}`),
		},
	}
	urSpec := kyvernov2.UpdateRequestSpec{
		RuleContext: []kyvernov2.RuleContext{
			{Trigger: spec},
		},
		Context: kyvernov2.UpdateRequestSpecContext{
			AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
				AdmissionRequest: admissionReq,
				Operation:        admissionv1.Create,
			},
		},
	}

	// getTriggerForCreateOperation calls GetResource then falls back to
	// admission request object when GetResource returns nil AND
	// SubResource is non-empty
	result, err := getTriggerForCreateOperation(client, urSpec, 0, logr.Discard())

	assert.NoError(t, err)
	assert.NotNil(t, result, "GetResource should fall back to admission request object for subresource creates")
	assert.Equal(t, "test-ns", result.GetName())
	assert.Equal(t, types.UID("phantom-uid-999"), result.GetUID())
	assert.Equal(t, "business", result.GetLabels()["layer"])
}

func TestGetResource_EmptySpec_FallbackToURResource(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "fallback-uid-777", nil)
	var emptySpec kyvernov1.ResourceSpec
	urSpec := kyvernov2.UpdateRequestSpec{
		Resource: kyvernov1.ResourceSpec{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       "test-ns",
			UID:        "fallback-uid-777",
		},
	}

	result, err := GetResource(client, emptySpec, urSpec, logr.Discard())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test-ns", result.GetName())
	assert.Equal(t, types.UID("fallback-uid-777"), result.GetUID())
}

// TestGetTrigger_DeleteOperation_ReturnsOldObject guards the delete-triggered
// generation path: a generate rule that explicitly matches the DELETE operation
// must still execute even though the trigger no longer exists in the cluster.
// The trigger is sourced from the admission request's oldObject and must not be
// subject to the UID liveness check applied to CREATE operations.
func TestGetTrigger_DeleteOperation_ReturnsOldObject(t *testing.T) {
	// no live namespace at all: the trigger was deleted
	client := &fakeListClient{Interface: dclient.NewEmptyFakeClient()}
	spec := makeNamespaceSpec("deleted-ns", "deleted-uid-123")
	urSpec := kyvernov2.UpdateRequestSpec{
		RuleContext: []kyvernov2.RuleContext{
			{Trigger: spec},
		},
		Context: kyvernov2.UpdateRequestSpecContext{
			AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
				AdmissionRequest: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Delete,
					OldObject: runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"deleted-ns","uid":"deleted-uid-123","labels":{"layer":"business"}}}`),
					},
				},
				Operation: admissionv1.Delete,
			},
		},
	}

	result, err := GetTrigger(client, urSpec, 0, logr.Discard())

	assert.NoError(t, err)
	assert.NotNil(t, result, "delete-triggered generation must receive the oldObject as trigger")
	assert.Equal(t, "deleted-ns", result.GetName())
	assert.Equal(t, types.UID("deleted-uid-123"), result.GetUID())
}

// TestGetResource_MutateExisting_PhantomUID verifies that the mutate-existing
// path (which resolves triggers through the same GetResource) also refuses to
// fall back to admission request bytes when the trigger UID does not match any
// live resource.
func TestGetResource_MutateExisting_PhantomUID(t *testing.T) {
	client := newFakeClientWithNamespace("test-ns", "live-uid-111", map[string]string{"layer": "operational"})
	spec := makeNamespaceSpec("test-ns", "phantom-uid-999")
	urSpec := kyvernov2.UpdateRequestSpec{
		Type: kyvernov2.Mutate,
		Context: kyvernov2.UpdateRequestSpecContext{
			AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
				AdmissionRequest: &admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"test-ns","uid":"phantom-uid-999","labels":{"layer":"business"}}}`),
					},
				},
				Operation: admissionv1.Create,
			},
		},
	}

	result, err := GetResource(client, spec, urSpec, logr.Discard())

	assert.Error(t, err)
	assert.Nil(t, result, "mutate-existing must not evaluate a phantom trigger from a rejected admission request")
}
