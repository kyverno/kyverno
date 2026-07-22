package admissionpolicygenerator

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
