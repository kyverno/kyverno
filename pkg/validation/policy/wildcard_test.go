package policy

import (
	"testing"

	kyverno "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/stretchr/testify/assert"
)

func Test_hasWildcardKinds(t *testing.T) {
	tests := []struct {
		name  string
		match kyverno.MatchResources
		want  bool
	}{
		{
			name: "kinds with wildcard star",
			match: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"*"},
				},
			},
			want: true,
		},
		{
			name: "kinds with specific resource",
			match: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"Pod"},
				},
			},
			want: false,
		},
		{
			name: "any filter with wildcard star",
			match: kyverno.MatchResources{
				Any: kyverno.ResourceFilters{
					{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"*"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "any filter with specific resource",
			match: kyverno.MatchResources{
				Any: kyverno.ResourceFilters{
					{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"Pod"},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "all filter with wildcard star",
			match: kyverno.MatchResources{
				All: kyverno.ResourceFilters{
					{
						ResourceDescription: kyverno.ResourceDescription{
							Kinds: []string{"*"},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "multiple kinds with one wildcard",
			match: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"Pod", "*"},
				},
			},
			want: true,
		},
		{
			name: "empty match",
			match: kyverno.MatchResources{},
			want:  false,
		},
		{
			name: "multiple specific kinds",
			match: kyverno.MatchResources{
				ResourceDescription: kyverno.ResourceDescription{
					Kinds: []string{"Pod", "Deployment"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasWildcardKinds(tt.match)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_checkWildcardMatchResources(t *testing.T) {
	tests := []struct {
		name    string
		spec    *kyverno.Spec
		wantMsg string
	}{
		{
			name: "policy with wildcard kinds should warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{
					{
						Name: "match-all",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"*"},
							},
						},
					},
				},
			},
			wantMsg: "Wildcard policy detected: this policy matches all resources which may add significant load to the API server",
		},
		{
			name: "policy with specific kinds should not warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{
					{
						Name: "match-pods",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"Pod"},
							},
						},
					},
				},
			},
			wantMsg: "",
		},
		{
			name: "policy with wildcard in any filter should warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{
					{
						Name: "match-all-any",
						MatchResources: kyverno.MatchResources{
							Any: kyverno.ResourceFilters{
								{
									ResourceDescription: kyverno.ResourceDescription{
										Kinds: []string{"*"},
									},
								},
							},
						},
					},
				},
			},
			wantMsg: "Wildcard policy detected: this policy matches all resources which may add significant load to the API server",
		},
		{
			name: "policy with Pod and wildcard should warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{
					{
						Name: "mixed-kinds",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"Pod", "*"},
							},
						},
					},
				},
			},
			wantMsg: "Wildcard policy detected: this policy matches all resources which may add significant load to the API server",
		},
		{
			name: "multiple rules with one wildcard should warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{
					{
						Name: "specific-rule",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"Pod"},
							},
						},
					},
					{
						Name: "wildcard-rule",
						MatchResources: kyverno.MatchResources{
							ResourceDescription: kyverno.ResourceDescription{
								Kinds: []string{"*"},
							},
						},
					},
				},
			},
			wantMsg: "Wildcard policy detected: this policy matches all resources which may add significant load to the API server",
		},
		{
			name: "empty rules should not warn",
			spec: &kyverno.Spec{
				Rules: []kyverno.Rule{},
			},
			wantMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkWildcardMatchResources(tt.spec)
			assert.Equal(t, tt.wantMsg, got)
		})
	}
}
