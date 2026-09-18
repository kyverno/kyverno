package generate

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/kyverno/kyverno/api/kyverno"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// failingDeleteClient wraps dclient.Interface and overrides DeleteResource to
// return a configured error, simulating non-404 failures such as RBAC denials
// or etcd timeouts.
type failingDeleteClient struct {
	dclient.Interface
	deleteErr error
}

func (f *failingDeleteClient) DeleteResource(_ context.Context, _, _, _, _ string, _ bool, _ metav1.DeleteOptions) error {
	return f.deleteErr
}

// fakeListDeleteClient is used to test handleNonPolicyChanges. ListResource
// returns pre-configured items so the deletion loop is exercised, and
// DeleteResource returns a configured error.
type fakeListDeleteClient struct {
	dclient.Interface
	deleteErr error
	listItems []unstructured.Unstructured
}

func (f *fakeListDeleteClient) DeleteResource(_ context.Context, _, _, _, _ string, _ bool, _ metav1.DeleteOptions) error {
	return f.deleteErr
}

func (f *fakeListDeleteClient) ListResource(_ context.Context, _, _, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	return &unstructured.UnstructuredList{Items: f.listItems}, nil
}

// TestDeleteDownstream_DeletionFails_ReturnsError is the targeted regression test
// for the bug. Before the fix, deleteDownstream called statusControl.Failed()
// internally but returned nil, silently swallowing the error. After the fix it
// returns the error so the caller can propagate it correctly.
func TestDeleteDownstream_DeletionFails_ReturnsError(t *testing.T) {
	controller := &GenerateController{
		client: &failingDeleteClient{
			Interface: dclient.NewEmptyFakeClient(),
			deleteErr: errors.New("etcd timeout: context deadline exceeded"),
		},
		log: logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
		Status: kyvernov2.UpdateRequestStatus{
			GeneratedResources: []kyvernov1.ResourceSpec{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "generated-cm"},
			},
		},
	}

	err := controller.deleteDownstream(nil, kyvernov2.RuleContext{}, ur)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clean up downstream resources on policy deletion")
}

// TestDeleteDownstream_NotFoundErrors_ReturnsNil verifies that 404 errors during
// deletion are treated as success — the resource is already gone so cleanup is
// considered complete.
func TestDeleteDownstream_NotFoundErrors_ReturnsNil(t *testing.T) {
	controller := &GenerateController{
		client: &failingDeleteClient{
			Interface: dclient.NewEmptyFakeClient(),
			deleteErr: apierrors.NewNotFound(schema.GroupResource{Resource: "configmaps"}, "already-gone"),
		},
		log: logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
		Status: kyvernov2.UpdateRequestStatus{
			GeneratedResources: []kyvernov1.ResourceSpec{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "already-gone"},
			},
		},
	}

	assert.NoError(t, controller.deleteDownstream(nil, kyvernov2.RuleContext{}, ur))
}

// TestDeleteDownstream_NoGeneratedResources_ReturnsNil verifies that when the UR
// has no GeneratedResources the function short-circuits and returns nil without
// touching the API server.
func TestDeleteDownstream_NoGeneratedResources_ReturnsNil(t *testing.T) {
	controller := &GenerateController{
		client: dclient.NewEmptyFakeClient(),
		log:    logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
		Status:     kyvernov2.UpdateRequestStatus{},
	}

	assert.NoError(t, controller.deleteDownstream(nil, kyvernov2.RuleContext{}, ur))
}

// TestHandleNonPolicyChanges_DeletionFails_ReturnsError tests that when
// downstream resources are found by label selector but deletion fails, the error
// is returned rather than swallowed.
func TestHandleNonPolicyChanges_DeletionFails_ReturnsError(t *testing.T) {
	downstream := unstructured.Unstructured{}
	downstream.SetAPIVersion("v1")
	downstream.SetKind("ConfigMap")
	downstream.SetNamespace("default")
	downstream.SetName("downstream-cm")

	controller := &GenerateController{
		client: &fakeListDeleteClient{
			Interface: dclient.NewEmptyFakeClient(),
			deleteErr: errors.New("forbidden: insufficient permissions"),
			listItems: []unstructured.Unstructured{downstream},
		},
		log: logr.Discard(),
	}

	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "sync-rule",
					Generation: &kyvernov1.Generation{
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								APIVersion: "v1",
								Kind:       "ConfigMap",
							},
						},
					},
				},
			},
		},
	}

	ruleContext := kyvernov2.RuleContext{
		Rule: "sync-rule",
		Trigger: kyvernov1.ResourceSpec{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       "test-ns",
		},
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
	}

	err := controller.handleNonPolicyChanges(policy, ruleContext, ur)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to clean up downstream resources on source deletion")
}

// TestProcessUR_DeleteDownstreamFailure_MarksURFailed is the end-to-end regression
// test. It proves the full call chain works correctly:
//
//	deleteDownstream error → appended to ProcessUR.failures → updateStatus(err)
//	→ statusControl.Failed()   (not statusControl.Success())
//
// On the old (buggy) code this test would FAIL: deleteDownstream returned nil,
// so ProcessUR called Success() and overwrote the Failed status.
func TestProcessUR_DeleteDownstreamFailure_MarksURFailed(t *testing.T) {
	statusControl := &fakeStatusControl{}
	policyLister := &fakeClusterPolicyLister{
		err: apierrors.NewNotFound(
			schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"},
			"deleted-policy",
		),
	}

	// Supply the trigger via an Update AdmissionRequest so GetTrigger extracts
	// it from the raw object without making any cluster call.
	triggerJSON := []byte(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"test-ns"}}`)

	controller := &GenerateController{
		client: &failingDeleteClient{
			Interface: dclient.NewEmptyFakeClient(),
			deleteErr: errors.New("etcd: request timed out"),
		},
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur", Namespace: "kyverno"},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "deleted-policy",
			RuleContext: []kyvernov2.RuleContext{
				{
					Rule: "generate-rule",
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       "Namespace",
						Name:       "test-ns",
					},
				},
			},
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object:    runtime.RawExtension{Raw: triggerJSON},
					},
					Operation: admissionv1.Update,
				},
			},
		},
		Status: kyvernov2.UpdateRequestStatus{
			GeneratedResources: []kyvernov1.ResourceSpec{
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "leaked-cm"},
			},
		},
	}

	_ = controller.ProcessUR(ur)

	assert.True(t, statusControl.failedCalled,
		"statusControl.Failed() must be called when downstream deletion fails")
	assert.False(t, statusControl.successCalled,
		"statusControl.Success() must NOT be called when downstream deletion fails — this is the core regression")
}

func cloneListDownstreamSecret(name, namespace, sourceUID string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					common.GeneratePolicyLabel:          "sync-secrets",
					common.GeneratePolicyNamespaceLabel: "",
					kyverno.LabelAppManagedBy:           kyverno.ValueKyvernoApp,
					// both downstreams share the same trigger (the target Namespace)
					common.GenerateTriggerGroupLabel:   "",
					common.GenerateTriggerVersionLabel: "v1",
					common.GenerateTriggerKindLabel:    "Namespace",
					common.GenerateTriggerNSLabel:      "",
					common.GenerateTriggerUIDLabel:     "trigger-uid-123",
					// each downstream references a distinct clone source
					common.GenerateSourceUIDLabel: sourceUID,
				},
			},
		},
	}
}

// TestHandleNonPolicyChanges_CloneListScopedToDeletedSource is the regression test
// for https://github.com/kyverno/kyverno/issues/9654 : when a single clone source
// is deleted, only the downstream cloned from that source is removed, not every
// downstream sharing the same trigger.
func TestHandleNonPolicyChanges_CloneListScopedToDeletedSource(t *testing.T) {
	ds1 := cloneListDownstreamSecret("dummy-secret-1", "certs-replicated", "source-uid-1")
	ds2 := cloneListDownstreamSecret("dummy-secret-2", "certs-replicated", "source-uid-2")

	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: "secrets"}:    "SecretList",
		{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
	}
	client, err := dclient.NewFakeClient(scheme, gvrToListKind, ds1, ds2)
	assert.NoError(t, err)
	client.SetDiscovery(dclient.NewFakeDiscoveryClient(nil))

	controller := &GenerateController{
		client: client,
		log:    logr.Discard(),
	}

	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "sync-secrets"},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "sync-secret",
					Generation: &kyvernov1.Generation{
						Synchronize: true,
						GeneratePattern: kyvernov1.GeneratePattern{
							CloneList: kyvernov1.CloneList{
								Namespace: "certs",
								Kinds:     []string{"v1/Secret"},
							},
						},
					},
				},
			},
		},
	}

	ruleContext := kyvernov2.RuleContext{
		Rule: "sync-secret",
		Trigger: kyvernov1.ResourceSpec{
			APIVersion: "v1",
			Kind:       "Namespace",
			Name:       "certs-replicated",
			UID:        "trigger-uid-123",
		},
	}

	// the deleted source secret (dummy-secret-1) carries the clone-source tag
	deletedSource := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      "dummy-secret-1",
			"namespace": "certs",
			"uid":       "source-uid-1",
			"labels": map[string]interface{}{
				common.GenerateTypeCloneSourceLabel: "",
			},
		},
	}
	raw, err := json.Marshal(deletedSource)
	assert.NoError(t, err)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
		Spec: kyvernov2.UpdateRequestSpec{
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					Operation: admissionv1.Delete,
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Delete,
						Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
						Namespace: "certs",
						OldObject: runtime.RawExtension{Raw: raw},
					},
				},
			},
		},
	}

	assert.NoError(t, controller.handleNonPolicyChanges(policy, ruleContext, ur))

	// dummy-secret-1's clone must be deleted
	_, err = client.GetResource(context.TODO(), "v1", "Secret", "certs-replicated", "dummy-secret-1")
	assert.True(t, apierrors.IsNotFound(err), "dummy-secret-1 clone should have been deleted")

	// dummy-secret-2's clone must be retained
	remaining, err := client.GetResource(context.TODO(), "v1", "Secret", "certs-replicated", "dummy-secret-2")
	assert.NoError(t, err, "dummy-secret-2 clone should be retained")
	assert.NotNil(t, remaining)
}

// secretRaw marshals a minimal Secret with the given uid and labels.
func secretRaw(t *testing.T, name, namespace, uid string, labels map[string]interface{}) []byte {
	t.Helper()
	meta := map[string]interface{}{"name": name, "namespace": namespace, "uid": uid}
	if labels != nil {
		meta["labels"] = labels
	}
	raw, err := json.Marshal(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata":   meta,
	})
	assert.NoError(t, err)
	return raw
}

// TestDeletedCloneSourceUID exercises every branch of the helper that decides
// whether a delete request must be scoped to a specific clone source.
func TestDeletedCloneSourceUID(t *testing.T) {
	controller := &GenerateController{log: logr.Discard()}

	newUR := func(req *admissionv1.AdmissionRequest) *kyvernov2.UpdateRequest {
		return &kyvernov2.UpdateRequest{
			Spec: kyvernov2.UpdateRequestSpec{
				Context: kyvernov2.UpdateRequestSpecContext{
					AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
						AdmissionRequest: req,
					},
				},
			},
		}
	}

	cloneSourceLabels := map[string]interface{}{common.GenerateTypeCloneSourceLabel: ""}

	tests := []struct {
		name string
		ur   *kyvernov2.UpdateRequest
		want string
	}{
		{
			name: "no admission request",
			ur:   newUR(nil),
			want: "",
		},
		{
			name: "non-delete operation",
			ur: newUR(&admissionv1.AdmissionRequest{
				Operation: admissionv1.Update,
				OldObject: runtime.RawExtension{Raw: secretRaw(t, "s", "certs", "uid-1", cloneSourceLabels)},
			}),
			want: "",
		},
		{
			name: "delete but old object is malformed",
			ur: newUR(&admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				OldObject: runtime.RawExtension{Raw: []byte("{not-json")},
			}),
			want: "",
		},
		{
			name: "delete of a non clone source (no clone-source label)",
			ur: newUR(&admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
				OldObject: runtime.RawExtension{Raw: secretRaw(t, "trigger", "certs", "uid-2", nil)},
			}),
			want: "",
		},
		{
			name: "delete of a clone source",
			ur: newUR(&admissionv1.AdmissionRequest{
				Operation: admissionv1.Delete,
				Kind:      metav1.GroupVersionKind{Version: "v1", Kind: "Secret"},
				OldObject: runtime.RawExtension{Raw: secretRaw(t, "dummy-secret-1", "certs", "source-uid-1", cloneSourceLabels)},
			}),
			want: "source-uid-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, controller.deletedCloneSourceUID(tt.ur))
		})
	}
}
