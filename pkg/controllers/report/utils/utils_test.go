package utils

import (
	"testing"

	"github.com/go-logr/logr"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

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

func TestGetExcludeReportingLabelRequirement(t *testing.T) {
	req, err := getExcludeReportingLabelRequirement()
	if err != nil {
		t.Fatalf("getExcludeReportingLabelRequirement() error: %v", err)
	}
	if req == nil {
		t.Fatal("getExcludeReportingLabelRequirement() returned nil requirement")
	}
}

func TestGetIncludeReportingLabelRequirement(t *testing.T) {
	req, err := getIncludeReportingLabelRequirement()
	if err != nil {
		t.Fatalf("getIncludeReportingLabelRequirement() error: %v", err)
	}
	if req == nil {
		t.Fatal("getIncludeReportingLabelRequirement() returned nil requirement")
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
