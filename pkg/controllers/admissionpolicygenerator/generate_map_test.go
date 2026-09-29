package admissionpolicygenerator

import (
	"context"
	"errors"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/kyverno/kyverno/pkg/auth/checker"
	"github.com/kyverno/kyverno/pkg/client/clientset/versioned/fake"
	policiesv1beta1listers "github.com/kyverno/kyverno/pkg/client/listers/policies.kyverno.io/v1beta1"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/utils/ptr"
)

// TestPreferredMAPVersion tests that the controller selects the right API version
// based on which listers are initialised, with v1beta1 taking precedence.
func TestPreferredMAPVersion(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*controller)
		wantVer admissionpolicy.MutatingAdmissionPolicyVersion
		wantOk  bool
	}{
		{
			name: "v1beta1 when both beta listers are set",
			setup: func(c *controller) {
				c.mapBetaLister = &mockMAPBetaLister{}
				c.mapbindingBetaLister = &mockMAPBindingBetaLister{}
			},
			wantVer: admissionpolicy.MutatingAdmissionPolicyVersionV1beta1,
			wantOk:  true,
		},
		{
			name: "v1alpha1 when both alpha listers are set",
			setup: func(c *controller) {
				c.mapAlphaLister = &mockMAPAlphaLister{}
				c.mapbindingAlphaLister = &mockMAPBindingAlphaLister{}
			},
			wantVer: admissionpolicy.MutatingAdmissionPolicyVersionV1alpha1,
			wantOk:  true,
		},
		{
			name: "v1beta1 preferred when all four listers are set",
			setup: func(c *controller) {
				c.mapBetaLister = &mockMAPBetaLister{}
				c.mapbindingBetaLister = &mockMAPBindingBetaLister{}
				c.mapAlphaLister = &mockMAPAlphaLister{}
				c.mapbindingAlphaLister = &mockMAPBindingAlphaLister{}
			},
			wantVer: admissionpolicy.MutatingAdmissionPolicyVersionV1beta1,
			wantOk:  true,
		},
		{
			name:    "not ok when only policy lister is set (binding missing)",
			setup:   func(c *controller) { c.mapBetaLister = &mockMAPBetaLister{} },
			wantVer: "",
			wantOk:  false,
		},
		{
			name:    "not ok when neither lister is set",
			setup:   func(c *controller) {},
			wantVer: "",
			wantOk:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{}
			tt.setup(c)
			got, ok := c.preferredMAPVersion()
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantVer, got)
		})
	}
}

// TestHandleMAPGeneration_NoLister verifies that handleMAPGeneration is a no-op
// when no MAP listers are configured (e.g. the MAP API is not available on the cluster).
func TestHandleMAPGeneration_NoLister(t *testing.T) {
	c := &controller{}
	err := c.handleMAPGeneration(context.Background(), &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
	})
	assert.NoError(t, err)
}

func mapGenEnabled() *policiesv1beta1.MutatingPolicyAutogenConfiguration {
	return &policiesv1beta1.MutatingPolicyAutogenConfiguration{
		MutatingAdmissionPolicy: &policiesv1beta1.MAPGenerationConfiguration{Enabled: ptr.To(true)},
	}
}

// TestMapGenerationSkipReason covers the decision of whether a MutatingAdmissionPolicy may be
// generated for a MutatingPolicy. A policy using useServerSideApply mutates atomic fields that a
// native MutatingAdmissionPolicy rejects, so generation must be skipped, otherwise the generated MAP
// becomes the sole admission path and the mutation breaks.
func TestMapGenerationSkipReason(t *testing.T) {
	tests := []struct {
		name       string
		policy     *policiesv1beta1.MutatingPolicy
		wantSkip   bool
		wantReason string
	}{
		{
			name: "generation not enabled",
			policy: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{},
			},
			wantSkip:   true,
			wantReason: "skip generating MutatingAdmissionPolicy: not enabled.",
		},
		{
			name: "generation enabled, plain mutation",
			policy: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					AutogenConfiguration: mapGenEnabled(),
				},
			},
			wantSkip: false,
		},
		{
			name: "generation enabled with useServerSideApply",
			policy: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					AutogenConfiguration: mapGenEnabled(),
					EvaluationConfiguration: &policiesv1beta1.MutatingPolicyEvaluationConfiguration{
						UseServerSideApply: true,
					},
				},
			},
			wantSkip:   true,
			wantReason: "skip generating MutatingAdmissionPolicy: useServerSideApply is enabled, which mutates atomic fields that a native MutatingAdmissionPolicy rejects.",
		},
		{
			name: "generation enabled with pod controllers autogen",
			policy: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					AutogenConfiguration: mapGenEnabled(),
				},
				Status: policiesv1beta1.MutatingPolicyStatus{
					Autogen: policiesv1beta1.MutatingPolicyAutogenStatus{
						Configs: map[string]policiesv1beta1.MutatingPolicyAutogen{"deployments": {}},
					},
				},
			},
			wantSkip:   true,
			wantReason: "skip generating MutatingAdmissionPolicy: pod controllers autogen is enabled.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := mapGenerationSkipReason(tt.policy)
			assert.Equal(t, tt.wantSkip, reason != "")
			if tt.wantReason != "" {
				assert.Equal(t, tt.wantReason, reason)
			}
		})
	}
}

// Mock implementations for listers

type mockMAPBetaLister struct{}

func (m *mockMAPBetaLister) List(selector labels.Selector) ([]*admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	return nil, nil
}

func (m *mockMAPBetaLister) Get(name string) (*admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	return nil, nil
}

type mockMAPBindingBetaLister struct{}

func (m *mockMAPBindingBetaLister) List(selector labels.Selector) ([]*admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, error) {
	return nil, nil
}

func (m *mockMAPBindingBetaLister) Get(name string) (*admissionregistrationv1beta1.MutatingAdmissionPolicyBinding, error) {
	return nil, nil
}

type mockCelPolexNamespaceListerForSuccess struct{}

func (m *mockCelPolexNamespaceListerForSuccess) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, nil
}
func (m *mockCelPolexNamespaceListerForSuccess) Get(name string) (*policiesv1beta1.PolicyException, error) {
	return nil, nil
}

type mockCelPolexListerForSuccess struct{}

func (m *mockCelPolexListerForSuccess) List(selector labels.Selector) ([]*policiesv1beta1.PolicyException, error) {
	return nil, nil
}
func (m *mockCelPolexListerForSuccess) PolicyExceptions(namespace string) policiesv1beta1listers.PolicyExceptionNamespaceLister {
	return &mockCelPolexNamespaceListerForSuccess{}
}

type mockMAPAlphaLister struct{}

func (m *mockMAPAlphaLister) List(selector labels.Selector) ([]*admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	return nil, nil
}

func (m *mockMAPAlphaLister) Get(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicy, error) {
	return nil, nil
}

type mockMAPBindingAlphaLister struct{}

func (m *mockMAPBindingAlphaLister) List(selector labels.Selector) ([]*admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
	return nil, nil
}

func (m *mockMAPBindingAlphaLister) Get(name string) (*admissionregistrationv1alpha1.MutatingAdmissionPolicyBinding, error) {
	return nil, nil
}

type mockMAPBetaListerForError struct{ err error }

func (m *mockMAPBetaListerForError) List(selector labels.Selector) ([]*admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	return nil, m.err
}
func (m *mockMAPBetaListerForError) Get(name string) (*admissionregistrationv1beta1.MutatingAdmissionPolicy, error) {
	return nil, m.err
}

type mockAuthChecker struct{}

func (m *mockAuthChecker) Check(ctx context.Context, group, version, resource, subresource, namespace, name, verb string) (*checker.AuthResult, error) {
	return &checker.AuthResult{Allowed: true}, nil
}

func TestHandleMAPGeneration_SentinelError(t *testing.T) {
	errSentinel := errors.New("sentinel map list error")
	c := &controller{
		client:               k8sfake.NewSimpleClientset(),
		kyvernoClient:        fake.NewSimpleClientset(),
		mapBetaLister:        &mockMAPBetaListerForError{err: errSentinel},
		mapbindingBetaLister: &mockMAPBindingBetaLister{},
		checker:              &mockAuthChecker{},
		celpolexLister:       &mockCelPolexListerForSuccess{},
	}

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints:     &admissionregistrationv1.MatchResources{},
			AutogenConfiguration: mapGenEnabled(),
		},
	}
	err := c.handleMAPGeneration(context.Background(), mpol)
	assert.ErrorIs(t, err, errSentinel)
}

func TestHandleMAPGeneration_APIError(t *testing.T) {
	errSentinel := errors.New("sentinel map api error")
	client := k8sfake.NewSimpleClientset()
	client.PrependReactor("create", "mutatingadmissionpolicies", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errSentinel
	})
	c := &controller{
		client:               client,
		kyvernoClient:        fake.NewSimpleClientset(),
		mapBetaLister:        &mockMAPBetaListerForError{err: apierrors.NewNotFound(schema.GroupResource{}, "notfound")},
		mapbindingBetaLister: &mockMAPBindingBetaLister{},
		checker:              &mockAuthChecker{},
		celpolexLister:       &mockCelPolexListerForSuccess{},
	}

	mpol := &policiesv1beta1.MutatingPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
		Spec: policiesv1beta1.MutatingPolicySpec{
			MatchConstraints:     &admissionregistrationv1.MatchResources{},
			AutogenConfiguration: mapGenEnabled(),
		},
	}
	err := c.handleMAPGeneration(context.Background(), mpol)
	assert.ErrorIs(t, err, errSentinel)
}
