package generate

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
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
