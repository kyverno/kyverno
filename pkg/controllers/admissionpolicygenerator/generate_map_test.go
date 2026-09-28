package admissionpolicygenerator

import (
	"context"
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	"github.com/kyverno/kyverno/pkg/admissionpolicy"
	mpolautogen "github.com/kyverno/kyverno/pkg/cel/policies/mpol/autogen"
	"github.com/stretchr/testify/assert"
	admissionregistrationv1alpha1 "k8s.io/api/admissionregistration/v1alpha1"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
)

// TestDesiredMAPVariants mirrors TestDesiredVAPVariants (generate_vap_test.go) for
// MutatingPolicy - see its doc comment for the scenario this covers.
func TestDesiredMAPVariants(t *testing.T) {
	baseSpec := &policiesv1beta1.MutatingPolicySpec{}

	tests := []struct {
		name        string
		configs     map[string]policiesv1beta1.MutatingPolicyAutogen
		wantNames   []string
		wantSkipped []string
	}{
		{
			name:      "no autogen: base variant only",
			wantNames: []string{"mpol-test"},
		},
		{
			name: "one translatable group",
			configs: map[string]policiesv1beta1.MutatingPolicyAutogen{
				"defaults": {Spec: baseSpec},
			},
			wantNames: []string{"mpol-test", "mpol-test-defaults"},
		},
		{
			name: "extraction-mode group only: no group variant, reported as skipped",
			configs: map[string]policiesv1beta1.MutatingPolicyAutogen{
				mpolautogen.ExtractionReplacementsRef: {Spec: baseSpec},
			},
			wantNames:   []string{"mpol-test"},
			wantSkipped: []string{mpolautogen.ExtractionReplacementsRef},
		},
		{
			name: "mixed: translatable group still generates even though a custom CRD is also autogen'd",
			configs: map[string]policiesv1beta1.MutatingPolicyAutogen{
				"cronjobs":                            {Spec: baseSpec},
				mpolautogen.ExtractionReplacementsRef: {Spec: baseSpec},
			},
			wantNames:   []string{"mpol-test", "mpol-test-cronjobs"},
			wantSkipped: []string{mpolautogen.ExtractionReplacementsRef},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mpol := &policiesv1beta1.MutatingPolicy{
				Status: policiesv1beta1.MutatingPolicyStatus{
					Autogen: policiesv1beta1.MutatingPolicyAutogenStatus{Configs: tt.configs},
				},
			}

			variants, skipped := desiredMAPVariants(mpol, "mpol-test")

			var gotNames []string
			for _, v := range variants {
				gotNames = append(gotNames, v.name)
				if v.name == "mpol-test" {
					assert.Nil(t, v.specOverride)
				} else {
					assert.NotNil(t, v.specOverride)
				}
			}
			assert.ElementsMatch(t, tt.wantNames, gotNames)
			assert.ElementsMatch(t, tt.wantSkipped, skipped)
		})
	}
}

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

// TestMapFullSkipReason covers the decision of whether a MutatingAdmissionPolicy must not be
// generated at all for a MutatingPolicy. A policy using useServerSideApply mutates atomic fields
// that a native MutatingAdmissionPolicy rejects, so generation must be skipped, otherwise the
// generated MAP becomes the sole admission path and the mutation breaks. Pod-controller autogen is
// no longer a full-skip reason (see TestDesiredMAPVariants) - it's now handled per-group by the
// fan-out generation in generate-map.go, which generates a MAP for every translatable group and
// only leaves out ones that genuinely can't be translated (custom-CRD/extraction-mode targets).
func TestMapFullSkipReason(t *testing.T) {
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
			// Previously this forced a full skip; now the fan-out in generate-map.go handles
			// autogen'd groups individually, so mapFullSkipReason no longer blocks on it.
			name: "generation enabled with pod controllers autogen is no longer a full-skip reason",
			policy: &policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					AutogenConfiguration: mapGenEnabled(),
				},
				Status: policiesv1beta1.MutatingPolicyStatus{
					Autogen: policiesv1beta1.MutatingPolicyAutogenStatus{
						Configs: map[string]policiesv1beta1.MutatingPolicyAutogen{"defaults": {}},
					},
				},
			},
			wantSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := mapFullSkipReason(tt.policy)
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
