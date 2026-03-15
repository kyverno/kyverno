package policy

import (
	"testing"

	policiesv1beta1 "github.com/kyverno/api/api/policies.kyverno.io/v1beta1"
	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	engineapi "github.com/kyverno/kyverno/pkg/engine/api"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func Test_hasWildcardInGVR(t *testing.T) {
	tests := []struct {
		name      string
		groups    []string
		versions  []string
		resources []string
		want      bool
	}{
		{"no wildcard", []string{"apps"}, []string{"v1"}, []string{"deployments"}, false},
		{"wildcard in group", []string{"*"}, []string{"v1"}, []string{"deployments"}, true},
		{"wildcard in version", []string{"apps"}, []string{"*"}, []string{"deployments"}, true},
		{"wildcard in resource", []string{"apps"}, []string{"v1"}, []string{"*"}, true},
		{"partial wildcard in resource", []string{"apps"}, []string{"v1"}, []string{"pod*"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasWildcardInGVR(tt.groups, tt.versions, tt.resources)
			if got != tt.want {
				t.Errorf("hasWildcardInGVR() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasWildcardInKyvernoMatch(t *testing.T) {
	tests := []struct {
		name  string
		match kyvernov1.MatchResources
		want  bool
	}{
		{
			name: "no wildcard kinds",
			match: kyvernov1.MatchResources{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod", "Deployment"},
				},
			},
			want: false,
		},
		{
			name: "wildcard in kinds",
			match: kyvernov1.MatchResources{
				ResourceDescription: kyvernov1.ResourceDescription{
					Kinds: []string{"Pod", "*"},
				},
			},
			want: true,
		},
		{
			name: "wildcard in any",
			match: kyvernov1.MatchResources{
				Any: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"*"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "wildcard in all",
			match: kyvernov1.MatchResources{
				All: kyvernov1.ResourceFilters{
					{
						ResourceDescription: kyvernov1.ResourceDescription{
							Kinds: []string{"*"},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasWildcardInKyvernoMatch(tt.match)
			if got != tt.want {
				t.Errorf("hasWildcardInKyvernoMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasWildcardInNativeRules(t *testing.T) {
	tests := []struct {
		name  string
		match *admissionregistrationv1.MatchResources
		want  bool
	}{
		{
			name:  "nil rules",
			match: nil,
			want:  false,
		},
		{
			name: "no wildcard",
			match: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"deployments"},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "wildcard in exclude",
			match: &admissionregistrationv1.MatchResources{
				ExcludeResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
					{
						RuleWithOperations: admissionregistrationv1.RuleWithOperations{
							Rule: admissionregistrationv1.Rule{
								APIGroups:   []string{"apps"},
								APIVersions: []string{"v1"},
								Resources:   []string{"*"},
							},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasWildcardInNativeRules(tt.match)
			if got != tt.want {
				t.Errorf("hasWildcardInNativeRules() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_HasWildcard(t *testing.T) {
	tests := []struct {
		name   string
		policy engineapi.GenericPolicy
		want   bool
	}{
		{
			name:   "nil policy",
			policy: nil,
			want:   false,
		},
		{
			name: "kyverno policy with wildcard",
			policy: engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							MatchResources: kyvernov1.MatchResources{
								ResourceDescription: kyvernov1.ResourceDescription{
									Kinds: []string{"*"},
								},
							},
						},
					},
				},
			}),
			want: true,
		},
		{
			name: "kyverno policy no wildcard",
			policy: engineapi.NewKyvernoPolicy(&kyvernov1.ClusterPolicy{
				Spec: kyvernov1.Spec{
					Rules: []kyvernov1.Rule{
						{
							MatchResources: kyvernov1.MatchResources{
								ResourceDescription: kyvernov1.ResourceDescription{
									Kinds: []string{"Pod"},
								},
							},
						},
					},
				},
			}),
			want: false,
		},
		{
			name: "validating admission policy with wildcard",
			policy: engineapi.NewValidatingAdmissionPolicy(&admissionregistrationv1.ValidatingAdmissionPolicy{
				Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
					MatchConstraints: &admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{
										APIGroups: []string{"*"},
									},
								},
							},
						},
					},
				},
			}),
			want: true,
		},
		{
			name: "validating policy with wildcard",
			policy: engineapi.NewValidatingPolicy(&policiesv1beta1.ValidatingPolicy{
				Spec: policiesv1beta1.ValidatingPolicySpec{
					MatchConstraints: ptr.To(admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{
										APIGroups: []string{"*"},
									},
								},
							},
						},
					}),
				},
			}),
			want: true,
		},
		{
			name: "mutating policy with wildcard",
			policy: engineapi.NewMutatingPolicy(&policiesv1beta1.MutatingPolicy{
				Spec: policiesv1beta1.MutatingPolicySpec{
					MatchConstraints: ptr.To(admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{
										APIGroups: []string{"*"},
									},
								},
							},
						},
					}),
				},
			}),
			want: true,
		},
		{
			name: "generating policy with wildcard",
			policy: engineapi.NewGeneratingPolicy(&policiesv1beta1.GeneratingPolicy{
				Spec: policiesv1beta1.GeneratingPolicySpec{
					MatchConstraints: ptr.To(admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{
										APIGroups: []string{"*"},
									},
								},
							},
						},
					}),
				},
			}),
			want: true,
		},
		{
			name: "image validating policy with wildcard",
			policy: engineapi.NewImageValidatingPolicy(&policiesv1beta1.ImageValidatingPolicy{
				Spec: policiesv1beta1.ImageValidatingPolicySpec{
					MatchConstraints: ptr.To(admissionregistrationv1.MatchResources{
						ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{
							{
								RuleWithOperations: admissionregistrationv1.RuleWithOperations{
									Rule: admissionregistrationv1.Rule{
										APIGroups: []string{"*"},
									},
								},
							},
						},
					}),
				},
			}),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasWildcard(tt.policy)
			if got != tt.want {
				t.Errorf("HasWildcard() = %v, want %v", got, tt.want)
			}
		})
	}
}
