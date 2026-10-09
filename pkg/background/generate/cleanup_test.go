package generate

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	"github.com/kyverno/kyverno/pkg/background/generate/provenance"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/config"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	target    *unstructured.Unstructured
}

func (f *failingDeleteClient) ListResource(_ context.Context, _, kind, _ string, _ *metav1.LabelSelector) (*unstructured.UnstructuredList, error) {
	if kind == "Namespace" {
		return &unstructured.UnstructuredList{Items: []unstructured.Unstructured{{Object: map[string]interface{}{"apiVersion": "v1", "kind": "Namespace", "metadata": map[string]interface{}{"name": "test-ns", "uid": "trigger-uid"}}}}}, nil
	}
	return &unstructured.UnstructuredList{}, nil
}

func (f *failingDeleteClient) GetResource(_ context.Context, _, _, _, _ string, _ ...string) (*unstructured.Unstructured, error) {
	return f.target.DeepCopy(), nil
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
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "generated-cm", UID: "generated-uid"},
			},
		},
	}

	authenticateCleanupRecord(t, controller, ur)
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
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "already-gone", UID: "deleted-uid"},
			},
		},
	}

	authenticateCleanupRecord(t, controller, ur)
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
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy", UID: "policy-uid"},
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
			UID:        "trigger-uid",
		},
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ur"},
	}

	trigger := unstructured.Unstructured{}
	trigger.SetAPIVersion("v1")
	trigger.SetKind("Namespace")
	trigger.SetName(ruleContext.Trigger.Name)
	trigger.SetUID(ruleContext.Trigger.UID)
	downstream.SetUID("downstream-uid")
	downstream.SetResourceVersion("1")
	common.ManageLabels(&downstream, trigger, policy, ruleContext.Rule)
	kube := controller.client.GetKubeClient()
	require.NoError(t, provenance.EnsureKey(t.Context(), kube.CoreV1().Secrets(config.KyvernoNamespace())))
	controller.provenance = provenance.NewStore(kube)
	stamp, err := controller.provenance.Sign(t.Context(), policy, &downstream)
	require.NoError(t, err)
	downstream.SetAnnotations(map[string]string{provenance.Annotation: stamp})
	controller.client.(*fakeListDeleteClient).listItems = []unstructured.Unstructured{downstream}

	err = controller.handleNonPolicyChanges(policy, ruleContext, ur)

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

	// Keep the admission context, while the fake client supplies the matching
	// persisted trigger UID used by the hardened UpdateRequest controller.
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
						UID:        "trigger-uid",
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
				{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "leaked-cm", UID: "generated-uid"},
			},
		},
	}

	authenticateCleanupRecord(t, controller, ur)
	_ = controller.ProcessUR(ur)

	assert.True(t, statusControl.failedCalled,
		"statusControl.Failed() must be called when downstream deletion fails")
	assert.False(t, statusControl.successCalled,
		"statusControl.Success() must NOT be called when downstream deletion fails — this is the core regression")
}

// A policy-deletion UR is trusted only after the policy controller has verified
// the downstream and recorded the original policy UID, before removing it.
func authenticateCleanupRecord(t *testing.T, controller *GenerateController, ur *kyvernov2.UpdateRequest) {
	t.Helper()
	ur.Spec.Policy = "deleted-policy"
	policy := &kyvernov1.ClusterPolicy{ObjectMeta: metav1.ObjectMeta{Name: ur.Spec.Policy, UID: "deleted-policy-uid"}}
	ur.SetAnnotations(map[string]string{provenance.CleanupPolicyUIDAnnotation: string(policy.UID)})
	kube := controller.client.GetKubeClient()
	require.NoError(t, provenance.EnsureKey(t.Context(), kube.CoreV1().Secrets(config.KyvernoNamespace())))
	controller.provenance = provenance.NewStore(kube)
	spec := ur.Status.GeneratedResources[0]
	target := &unstructured.Unstructured{}
	target.SetAPIVersion(spec.APIVersion)
	target.SetKind(spec.Kind)
	target.SetNamespace(spec.Namespace)
	target.SetName(spec.Name)
	target.SetUID(spec.UID)
	target.SetResourceVersion("1")
	trigger := unstructured.Unstructured{}
	trigger.SetAPIVersion("v1")
	trigger.SetKind("Namespace")
	trigger.SetName("trigger")
	trigger.SetUID("trigger-uid")
	common.ManageLabels(target, trigger, policy, "generate-rule")
	stamp, err := controller.provenance.Sign(t.Context(), policy, target)
	require.NoError(t, err)
	target.SetAnnotations(map[string]string{provenance.Annotation: stamp})
	controller.client.(*failingDeleteClient).target = target
}
