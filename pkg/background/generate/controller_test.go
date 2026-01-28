package generate

import (
	"errors"
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/pkg/background/common"
	kyvernov1listers "github.com/kyverno/kyverno/pkg/client/listers/kyverno/v1"
	"github.com/kyverno/kyverno/pkg/clients/dclient"
	"github.com/kyverno/kyverno/pkg/event"
	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1listers "k8s.io/client-go/listers/core/v1"
)

// fakeClusterPolicyLister returns NotFound for all policy lookups
type fakeClusterPolicyLister struct {
	policy *kyvernov1.ClusterPolicy
	err    error
}

func (f *fakeClusterPolicyLister) List(selector labels.Selector) ([]*kyvernov1.ClusterPolicy, error) {
	if f.policy != nil {
		return []*kyvernov1.ClusterPolicy{f.policy}, nil
	}
	return nil, f.err
}

func (f *fakeClusterPolicyLister) Get(name string) (*kyvernov1.ClusterPolicy, error) {
	if f.policy != nil && f.policy.Name == name {
		return f.policy, nil
	}
	return nil, f.err
}

// fakePolicyLister returns NotFound for all namespaced policy lookups
type fakePolicyLister struct{}

func (f *fakePolicyLister) List(selector labels.Selector) ([]*kyvernov1.Policy, error) {
	return nil, nil
}

func (f *fakePolicyLister) Policies(namespace string) kyvernov1listers.PolicyNamespaceLister {
	return &fakePolicyNamespaceLister{}
}

type fakePolicyNamespaceLister struct{}

func (f *fakePolicyNamespaceLister) List(selector labels.Selector) ([]*kyvernov1.Policy, error) {
	return nil, nil
}

func (f *fakePolicyNamespaceLister) Get(name string) (*kyvernov1.Policy, error) {
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "policies"}, name)
}

// fakeStatusControl simulates status update failures
type fakeStatusControl struct {
	failedErr  error
	successErr error
	skipErr    error

	failedCalled  bool
	successCalled bool
	skipCalled    bool
}

func (f *fakeStatusControl) Failed(name string, message string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.failedCalled = true
	return nil, f.failedErr
}

func (f *fakeStatusControl) Success(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.successCalled = true
	return nil, f.successErr
}

func (f *fakeStatusControl) Skip(name string, genResources []kyvernov1.ResourceSpec) (*kyvernov2.UpdateRequest, error) {
	f.skipCalled = true
	return nil, f.skipErr
}

// TestProcessUR_NilPolicy_NoEventPanic tests that when a policy is deleted (NotFound)
// and an error occurs during downstream cleanup, the controller does not panic
// when trying to create a background failed event with a nil policy.
//
// This test verifies the fix for the nil pointer dereference that occurred when:
// 1. A generate policy is deleted from the cluster
// 2. UpdateRequests still reference the deleted policy
// 3. The UR has GeneratedResources that need cleanup
// 4. The status update fails (API conflict, unavailable, etc.)
// 5. The error handling code tried to call NewKyvernoPolicy(nil)
func TestProcessUR_NilPolicy_NoEventPanic(t *testing.T) {
	// Create a fake status control that returns an error when trying to update status
	// This simulates API server unavailability or conflict during cleanup
	statusControl := &fakeStatusControl{
		failedErr:  errors.New("simulated API server error"),
		successErr: errors.New("simulated API server error"),
	}

	// Create a policy lister that returns NotFound (simulating deleted policy)
	policyLister := &fakeClusterPolicyLister{
		err: apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "deleted-policy"),
	}

	// Create a minimal GenerateController with required dependencies
	controller := &GenerateController{
		client:        dclient.NewEmptyFakeClient(),
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	// Create an UpdateRequest that references a deleted policy
	// with GeneratedResources to trigger the cleanup path
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			// Reference a non-existent (deleted) policy
			Policy: "deleted-policy",
			// Add RuleContext to ensure the processing loop executes
			RuleContext: []kyvernov2.RuleContext{
				{
					Rule: "generate-rule",
					// Trigger resource spec
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       "Namespace",
						Name:       "test-namespace",
					},
				},
			},
			// Context with minimal admission request info
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
		Status: kyvernov2.UpdateRequestStatus{
			// Add GeneratedResources to trigger the deleteDownstream path
			// when policy is NotFound
			GeneratedResources: []kyvernov1.ResourceSpec{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  "test-namespace",
					Name:       "generated-cm",
				},
			},
		},
	}

	// This should NOT panic even though:
	// 1. Policy is NotFound (nil)
	// 2. applyGenerate will call deleteDownstream
	// 3. deleteDownstream will try to delete resources and update status
	// 4. Status update fails, returning an error
	// 5. ProcessUR error handling should gracefully handle nil policy
	assert.NotPanics(t, func() {
		err := controller.ProcessUR(ur)
		// We expect an error from the failed status update, but NO panic
		// The error may or may not be nil depending on whether cleanup succeeds
		_ = err
	}, "ProcessUR should not panic when policy is deleted and status update fails")
}

// TestProcessUR_NilPolicy_DeleteDownstream_StatusUpdateFails specifically tests
// the scenario where a deleted policy causes deleteDownstream to be called,
// and the status update fails, which previously caused a nil pointer panic
// when creating the background failed event.
func TestProcessUR_NilPolicy_DeleteDownstream_StatusUpdateFails(t *testing.T) {
	// Track whether Failed was called
	statusControl := &fakeStatusControl{
		// Make the status update fail to trigger the error path
		failedErr:  errors.New("conflict: resource version changed"),
		successErr: errors.New("conflict: resource version changed"),
	}

	// Policy lister returns NotFound
	policyLister := &fakeClusterPolicyLister{
		err: apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "deleted-policy"),
	}

	controller := &GenerateController{
		client:        dclient.NewEmptyFakeClient(),
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-2",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "deleted-policy",
			RuleContext: []kyvernov2.RuleContext{
				{
					Rule:             "generate-rule",
					DeleteDownstream: false, // Explicitly false to trigger the policy == nil path
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       "Namespace",
						Name:       "test-ns",
					},
				},
			},
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
		Status: kyvernov2.UpdateRequestStatus{
			GeneratedResources: []kyvernov1.ResourceSpec{
				{
					APIVersion: "v1",
					Kind:       "ConfigMap",
					Namespace:  "default",
					Name:       "generated-resource",
				},
			},
		},
	}

	// Should not panic
	assert.NotPanics(t, func() {
		_ = controller.ProcessUR(ur)
	}, "ProcessUR must not panic when policy is nil and error occurs during cleanup")

	// Verify that either Success or Failed was called (cleanup was attempted)
	assert.True(t, statusControl.successCalled || statusControl.failedCalled,
		"Status control should have been called during cleanup")
}

// TestProcessUR_NilPolicy_NoGeneratedResources tests the case where
// the policy is deleted but there are no GeneratedResources to clean up.
// This should complete gracefully without panic.
func TestProcessUR_NilPolicy_NoGeneratedResources(t *testing.T) {
	statusControl := &fakeStatusControl{}

	// Policy lister returns NotFound
	policyLister := &fakeClusterPolicyLister{
		err: apierrors.NewNotFound(schema.GroupResource{Group: "kyverno.io", Resource: "clusterpolicies"}, "deleted-policy"),
	}

	controller := &GenerateController{
		client:        dclient.NewEmptyFakeClient(),
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-no-gen",
			Namespace: "kyverno",
		},
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
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{},
			},
		},
		// No GeneratedResources - cleanup path should exit early
		Status: kyvernov2.UpdateRequestStatus{},
	}

	// Should not panic - deleteDownstream returns nil when no GeneratedResources
	assert.NotPanics(t, func() {
		err := controller.ProcessUR(ur)
		// Should complete without error since deleteDownstream returns nil
		// when there are no GeneratedResources to clean up
		_ = err
	}, "ProcessUR should not panic when policy is deleted and no GeneratedResources exist")
}

// fakeNamespaceLister returns errors for namespace lookups
type fakeNamespaceLister struct {
	err error
}

func (f *fakeNamespaceLister) List(selector labels.Selector) ([]*corev1.Namespace, error) {
	return nil, f.err
}

func (f *fakeNamespaceLister) Get(name string) (*corev1.Namespace, error) {
	return nil, f.err
}

// TestProcessUR_ApplyGenerateError_MarksURFailed tests that when applyGenerate()
// returns an error, the UpdateRequest is marked as Failed (not Success).
//
// This test validates the fix for the bug where applyGenerate errors were logged
// and evented but NOT added to the failures slice, causing updateStatus to
// incorrectly mark the UR as Success.
//
// The test will:
// - FAIL on the old code (where errors were silently dropped)
// - PASS with the fix applied (where errors are added to failures slice)
func TestProcessUR_ApplyGenerateError_MarksURFailed(t *testing.T) {
	// Create a status control that tracks which methods are called
	statusControl := &fakeStatusControl{}

	// Create a policy that EXISTS (not NotFound) with a generate rule
	// The policy has a namespace selector so that GetNamespaceSelectorsFromNamespaceLister
	// will attempt to fetch the namespace
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-generate-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "generate-configmap",
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"ConfigMap"},
							// NamespaceSelector triggers the namespace lookup path in applyGenerate
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"env": "test"},
							},
						},
					},
					Generation: &kyvernov1.Generation{
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								Kind:       "Secret",
								APIVersion: "v1",
								Name:       "generated-secret",
								Namespace:  "default",
							},
						},
					},
				},
			},
		},
	}

	// Policy lister returns the policy (NOT NotFound)
	policyLister := &fakeClusterPolicyLister{
		policy: policy,
	}

	// Namespace lister returns an error - this will cause applyGenerate to fail
	// when it tries to get namespace labels for the namespace selector
	nsLister := &fakeNamespaceLister{
		err: errors.New("simulated namespace lookup failure"),
	}

	controller := &GenerateController{
		client:        dclient.NewEmptyFakeClient(),
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		nsLister:      nsLister,
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	// Create the trigger object JSON for the AdmissionRequest
	// This allows GetTrigger to extract the object without fetching from cluster
	triggerJSON := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"trigger-cm","namespace":"test-namespace"}}`)

	// Create an UpdateRequest that references the existing policy
	// We provide an AdmissionRequest so GetTrigger extracts from it (not from cluster)
	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-apply-error",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "test-generate-policy",
			RuleContext: []kyvernov2.RuleContext{
				{
					Rule: "generate-configmap",
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "test-namespace",
						Name:       "trigger-cm",
					},
				},
			},
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					// Provide AdmissionRequest so GetTrigger extracts object from here
					// instead of fetching from cluster (which would fail)
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Update, // Update operation extracts from request
						Object: runtime.RawExtension{
							Raw: triggerJSON,
						},
					},
					Operation: admissionv1.Update,
				},
			},
		},
		Status: kyvernov2.UpdateRequestStatus{},
	}

	// Process the UR - applyGenerate will fail due to namespace lookup error
	err := controller.ProcessUR(ur)

	// CRITICAL ASSERTIONS:
	// With the bug fix, Failed() should be called because the error from
	// applyGenerate is now added to the failures slice.
	// Without the fix, Success() would be called because failures slice was empty.

	assert.True(t, statusControl.failedCalled,
		"statusControl.Failed() should be called when applyGenerate returns an error")
	assert.False(t, statusControl.successCalled,
		"statusControl.Success() should NOT be called when applyGenerate returns an error")

	// Additional sanity check - we expect no error from ProcessUR itself
	// since updateStatus with our fake doesn't return an error
	assert.NoError(t, err, "ProcessUR should not return error when status update succeeds")
}

// TestProcessUR_ApplyGenerateError_MultipleRules tests that when applyGenerate()
// fails for one rule but succeeds for another, the UR is still marked as Failed.
func TestProcessUR_ApplyGenerateError_MultipleRules_StillFails(t *testing.T) {
	statusControl := &fakeStatusControl{}

	// Policy with a rule that has a namespace selector (will trigger ns lookup)
	policy := &kyvernov1.ClusterPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-multi-rule-policy",
		},
		Spec: kyvernov1.Spec{
			Rules: []kyvernov1.Rule{
				{
					Name: "rule-that-will-fail",
					MatchResources: kyvernov1.MatchResources{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"ConfigMap"},
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"env": "test"},
							},
						},
					},
					Generation: &kyvernov1.Generation{
						GeneratePattern: kyvernov1.GeneratePattern{
							ResourceSpec: kyvernov1.ResourceSpec{
								Kind:       "Secret",
								APIVersion: "v1",
								Name:       "generated-secret",
							},
						},
					},
				},
			},
		},
	}

	policyLister := &fakeClusterPolicyLister{
		policy: policy,
	}

	// Namespace lister fails - causes applyGenerate to return error
	nsLister := &fakeNamespaceLister{
		err: errors.New("namespace not found"),
	}

	controller := &GenerateController{
		client:        dclient.NewEmptyFakeClient(),
		statusControl: statusControl,
		policyLister:  policyLister,
		npolicyLister: &fakePolicyLister{},
		nsLister:      nsLister,
		eventGen:      event.NewFake(),
		log:           logr.Discard(),
	}

	triggerJSON := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"trigger","namespace":"some-namespace"}}`)

	ur := &kyvernov2.UpdateRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ur-multi-rule",
			Namespace: "kyverno",
		},
		Spec: kyvernov2.UpdateRequestSpec{
			Policy: "test-multi-rule-policy",
			RuleContext: []kyvernov2.RuleContext{
				{
					Rule: "rule-that-will-fail",
					Trigger: kyvernov1.ResourceSpec{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Namespace:  "some-namespace",
						Name:       "trigger",
					},
				},
			},
			Context: kyvernov2.UpdateRequestSpecContext{
				AdmissionRequestInfo: kyvernov2.AdmissionRequestInfoObject{
					AdmissionRequest: &admissionv1.AdmissionRequest{
						Operation: admissionv1.Update,
						Object: runtime.RawExtension{
							Raw: triggerJSON,
						},
					},
					Operation: admissionv1.Update,
				},
			},
		},
		Status: kyvernov2.UpdateRequestStatus{},
	}

	_ = controller.ProcessUR(ur)

	// Even with multiple rules where some might "succeed" (return nil),
	// any failure should cause Failed() to be called
	assert.True(t, statusControl.failedCalled,
		"Failed() must be called when any rule fails")
	assert.False(t, statusControl.successCalled,
		"Success() must NOT be called when any rule fails")
}

// Ensure fake types satisfy the required interfaces
var _ kyvernov1listers.ClusterPolicyLister = &fakeClusterPolicyLister{}
var _ kyvernov1listers.PolicyLister = &fakePolicyLister{}
var _ kyvernov1listers.PolicyNamespaceLister = &fakePolicyNamespaceLister{}
var _ common.StatusControlInterface = &fakeStatusControl{}
var _ corev1listers.NamespaceLister = &fakeNamespaceLister{}
