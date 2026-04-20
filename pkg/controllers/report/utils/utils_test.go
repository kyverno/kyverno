package utils

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	reportsv1 "github.com/kyverno/kyverno/api/reports/v1"
	"github.com/kyverno/kyverno/pkg/openreports"
	openreportsv1alpha1 "github.com/openreports/reports-api/apis/openreports.io/v1alpha1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestReportsAreIdentical_EmptyReports(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	after := &reportsv1.EphemeralReport{}

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "empty reports should be identical")
}

func TestReportsAreIdentical_SameResults(t *testing.T) {
	results := []openreportsv1alpha1.ReportResult{
		{
			Policy: "test-policy",
			Rule:   "test-rule",
			Result: openreportsv1alpha1.Result(openreports.StatusPass),
		},
	}
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	before.SetResults(results)

	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "report1",
			Labels:      map[string]string{"app": "test"},
			Annotations: map[string]string{"key": "value"},
		},
	}
	after.SetResults(results)

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same results should be identical")
}

func TestReportsAreIdentical_DifferentResults(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusFail)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different results should not be identical")
}

func TestReportsAreIdentical_DifferentResultCount(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{Policy: "policy1", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
		{Policy: "policy2", Result: openreportsv1alpha1.Result(openreports.StatusPass)},
	})

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different result count should not be identical")
}

func TestReportsAreIdentical_DifferentLabels(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "test"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "different"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different labels should not be identical")
}

func TestReportsAreIdentical_DifferentAnnotations(t *testing.T) {
	before := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value1"},
		},
	}
	after := &reportsv1.EphemeralReport{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{"key": "value2"},
		},
	}

	result := ReportsAreIdentical(before, after)
	assert.False(t, result, "reports with different annotations should not be identical")
}

func TestReportsAreIdentical_TimestampIgnored(t *testing.T) {
	before := &reportsv1.EphemeralReport{}
	before.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 1000},
		},
	})

	after := &reportsv1.EphemeralReport{}
	after.SetResults([]openreportsv1alpha1.ReportResult{
		{
			Policy:    "policy1",
			Result:    openreportsv1alpha1.Result(openreports.StatusPass),
			Timestamp: metav1.Timestamp{Seconds: 2000},
		},
	})

	result := ReportsAreIdentical(before, after)
	assert.True(t, result, "reports with same content but different timestamps should be identical")
}

func TestGetExcludeReportingLabelRequirement(t *testing.T) {
	req, err := getExcludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestGetIncludeReportingLabelRequirement(t *testing.T) {
	req, err := getIncludeReportingLabelRequirement()
	assert.NoError(t, err)
	assert.NotNil(t, req)
}

func TestRemoveNonValidationPolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies []kyvernov1.PolicyInterface
		want     int
	}{
		{
			name: "filter validation policies",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Validation: &kyvernov1.Validation{
									Message: "test validation",
								},
							},
						},
					},
				},
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Mutation: &kyvernov1.Mutation{},
							},
						},
					},
				},
			},
			want: 1,
		},
		{
			name:     "empty list",
			policies: []kyvernov1.PolicyInterface{},
			want:     0,
		},
		{
			name:     "nil list",
			policies: nil,
			want:     0,
		},
		{
			name: "policy with verify images",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								VerifyImages: []kyvernov1.ImageVerification{
									{
										ImageReferences: []string{"ghcr.io/*"},
									},
								},
							},
						},
					},
				},
			},
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveNonValidationPolicies(tt.policies...)
			if len(result) != tt.want {
				t.Errorf("RemoveNonValidationPolicies() returned %d policies, want %d", len(result), tt.want)
			}
		})
	}
}

func TestBuildKindSet(t *testing.T) {
	tests := []struct {
		name     string
		policies []kyvernov1.PolicyInterface
		want     sets.Set[string]
	}{
		{
			name: "policy with validation rule",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "test-policy"},
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Name: "test-rule",
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod", "Deployment"},
									},
								},
								Validation: &kyvernov1.Validation{
									Message: "test",
								},
							},
						},
					},
				},
			},
			want: sets.New("Pod", "Deployment"),
		},
		{
			name:     "empty policies",
			policies: []kyvernov1.PolicyInterface{},
			want:     sets.New[string](),
		},
		{
			name: "policy with mutation only - no kinds extracted",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Name: "mutate-rule",
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"ConfigMap"},
									},
								},
								Mutation: &kyvernov1.Mutation{},
							},
						},
					},
				},
			},
			want: sets.New[string](),
		},
		{
			name: "multiple policies with overlapping kinds",
			policies: []kyvernov1.PolicyInterface{
				&kyvernov1.ClusterPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy-1"},
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Name: "rule-1",
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod", "Service"},
									},
								},
								Validation: &kyvernov1.Validation{Message: "test"},
							},
						},
					},
				},
				&kyvernov1.ClusterPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "policy-2"},
					Spec: kyvernov1.Spec{
						Rules: []kyvernov1.Rule{
							{
								Name: "rule-2",
								MatchResources: kyvernov1.MatchResources{
									ResourceDescription: kyvernov1.ResourceDescription{
										Kinds: []string{"Pod", "Deployment"},
									},
								},
								Validation: &kyvernov1.Validation{Message: "test2"},
							},
						},
					},
				},
			},
			want: sets.New("Pod", "Service", "Deployment"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildKindSet(logr.Discard(), tt.policies...)

			if result.Len() != tt.want.Len() {
				t.Errorf("BuildKindSet() returned %d kinds, want %d", result.Len(), tt.want.Len())
			}

			for kind := range tt.want {
				if !result.Has(kind) {
					t.Errorf("BuildKindSet() missing expected kind %q", kind)
				}
			}
		})
	}
}

func TestRemoveNonBackgroundPolicies(t *testing.T) {
	tests := []struct {
		name     string
		policies []kyvernov1.PolicyInterface
		wantLen  int
	}{
		{
			name:     "empty list",
			policies: []kyvernov1.PolicyInterface{},
			wantLen:  0,
		},
		{
			name:     "nil list",
			policies: nil,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveNonBackgroundPolicies(tt.policies...)
			if len(result) != tt.wantLen {
				t.Errorf("RemoveNonBackgroundPolicies() returned %d policies, want %d", len(result), tt.wantLen)
			}
		})
	}
}
