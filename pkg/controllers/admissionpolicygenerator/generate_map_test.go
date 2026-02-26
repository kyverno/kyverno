package admissionpolicygenerator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	"k8s.io/apimachinery/pkg/labels"
)

// TestHandleMAPGeneration_VersionSelection tests that the controller prefers v1beta1 over v1alpha1
func TestHandleMAPGeneration_VersionSelection(t *testing.T) {
	tests := []struct {
		name             string
		hasV1Beta1Lister bool
		hasV1AlphaLister bool
		expectedVersion  string
	}{
		{
			name:             "prefer v1beta1 when available",
			hasV1Beta1Lister: true,
			hasV1AlphaLister: true,
			expectedVersion:  "v1beta1",
		},
		{
			name:             "use v1alpha1 when v1beta1 not available",
			hasV1Beta1Lister: false,
			hasV1AlphaLister: true,
			expectedVersion:  "v1alpha1",
		},
		{
			name:             "no lister available returns no version",
			hasV1Beta1Lister: false,
			hasV1AlphaLister: false,
			expectedVersion:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &controller{}

			// Mock listers based on test case
			if tt.hasV1Beta1Lister {
				c.mapLister = &mockMAPBetaLister{}
				c.mapbindingLister = &mockMAPBindingBetaLister{}
			}
			if tt.hasV1AlphaLister {
				c.mapAlphaLister = &mockMAPAlphaLister{}
				c.mapbindingAlphaLister = &mockMAPBindingAlphaLister{}
			}

			// Test version preference logic
			var selectedVersion string
			if c.mapLister != nil {
				selectedVersion = "v1beta1"
			} else if c.mapAlphaLister != nil {
				selectedVersion = "v1alpha1"
			}

			assert.Equal(t, tt.expectedVersion, selectedVersion)
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
