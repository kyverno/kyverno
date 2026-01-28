package utils

import (
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
)

func TestMatchDeleteOperation(t *testing.T) {
	tests := []struct {
		name string
		rule kyvernov1.Rule
		want bool
	}{
		{
			name: "delete operation in top-level match",
			rule: kyvernov1.Rule{
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Operations: []kyvernov1.AdmissionOperation{
							kyvernov1.Delete,
						},
					},
				},
			},
			want: true,
		},
		{
			name: "delete operation in match.any",
			rule: kyvernov1.Rule{
				MatchResources: kyvernov1.MatchResources{
					Any: []kyvernov1.ResourceFilter{
						{
							ResourceDescription: kyvernov1.ResourceDescription{
								Operations: []kyvernov1.AdmissionOperation{
									kyvernov1.Delete,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "delete operation in match.all",
			rule: kyvernov1.Rule{
				MatchResources: kyvernov1.MatchResources{
					All: []kyvernov1.ResourceFilter{
						{
							ResourceDescription: kyvernov1.ResourceDescription{
								Operations: []kyvernov1.AdmissionOperation{
									kyvernov1.Delete,
								},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "non-delete operation",
			rule: kyvernov1.Rule{
				MatchResources: kyvernov1.MatchResources{
					ResourceDescription: kyvernov1.ResourceDescription{
						Operations: []kyvernov1.AdmissionOperation{
							kyvernov1.Create,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "no operations defined",
			rule: kyvernov1.Rule{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchDeleteOperation(tt.rule)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
